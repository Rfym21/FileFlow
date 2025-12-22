// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package webdav

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

// Handler is a WebDAV request handler.
type Handler struct {
	// Prefix is the URL path prefix to strip from WebDAV resource paths.
	Prefix string
	// LockSystem is the lock management system.
	LockSystem LockSystem
	// Logger is an optional error logger.
	Logger func(*http.Request, error)
}

// stripPrefix removes the Handler's prefix from the request's URL path.
func (h *Handler) stripPrefix(p string) (string, int, error) {
	if h.Prefix == "" {
		return p, http.StatusOK, nil
	}
	if r := strings.TrimPrefix(p, h.Prefix); len(r) < len(p) {
		return r, http.StatusOK, nil
	}
	return p, http.StatusNotFound, errPrefixMismatch
}

func (h *Handler) log(r *http.Request, err error) {
	if h.Logger != nil {
		h.Logger(r, err)
	}
}

// ServeHTTP dispatches the request to the handler whose pattern matches the request URL.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status, err := http.StatusBadRequest, errNoFileSystem

	// Get storage and user from context
	storage, ok := r.Context().Value(storageKey).(Storage)
	if !ok {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	user, ok := r.Context().Value(userKey).(User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Add user agent to context
	ctx := context.WithValue(r.Context(), userAgentKey, r.UserAgent())
	r = r.WithContext(ctx)

	switch r.Method {
	case "OPTIONS":
		status, err = h.handleOptions(w, r)
	case "GET", "HEAD":
		status, err = h.handleGetHead(w, r, storage, user)
	case "DELETE":
		status, err = h.handleDelete(w, r, storage, user)
	case "PUT":
		status, err = h.handlePut(w, r, storage, user)
	case "MKCOL":
		status, err = h.handleMkcol(w, r, storage, user)
	case "COPY", "MOVE":
		status, err = h.handleCopyMove(w, r, storage, user)
	case "LOCK":
		status, err = h.handleLock(w, r, user)
	case "UNLOCK":
		status, err = h.handleUnlock(w, r, user)
	case "PROPFIND":
		status, err = h.handlePropfind(w, r, storage, user)
	case "PROPPATCH":
		status, err = h.handleProppatch(w, r, storage, user)
	}

	if status != 0 {
		w.WriteHeader(status)
		if status != http.StatusNoContent {
			w.Write([]byte(StatusText(status)))
		}
	}
	if err != nil {
		h.log(r, err)
	}
}

// StatusText returns a text for the HTTP status code.
func StatusText(code int) string {
	switch code {
	case StatusMulti:
		return "Multi-Status"
	case StatusUnprocessable:
		return "Unprocessable Entity"
	case StatusLocked:
		return "Locked"
	case StatusFailedDependency:
		return "Failed Dependency"
	case StatusInsufficientStorage:
		return "Insufficient Storage"
	}
	return http.StatusText(code)
}

func (h *Handler) handleOptions(w http.ResponseWriter, r *http.Request) (status int, err error) {
	reqPath, status, err := h.stripPrefix(r.URL.Path)
	if err != nil {
		return status, err
	}
	ctx := r.Context()
	allow := "OPTIONS, LOCK, PUT, MKCOL"
	storage, ok := ctx.Value(storageKey).(Storage)
	if ok {
		if fi, err := storage.Get(ctx, reqPath); err == nil {
			if fi.IsDir() {
				allow = "OPTIONS, LOCK, DELETE, PROPPATCH, COPY, MOVE, UNLOCK, PROPFIND"
			} else {
				allow = "OPTIONS, LOCK, GET, HEAD, DELETE, PROPPATCH, COPY, MOVE, UNLOCK, PROPFIND, PUT"
			}
		}
	}
	w.Header().Set("Allow", allow)
	w.Header().Set("DAV", "1, 2")
	w.Header().Set("MS-Author-Via", "DAV")
	return 0, nil
}

func (h *Handler) handleGetHead(w http.ResponseWriter, r *http.Request, storage Storage, user User) (status int, err error) {
	reqPath, status, err := h.stripPrefix(r.URL.Path)
	if err != nil {
		return status, err
	}
	if !user.CanWebdavRead() {
		return http.StatusForbidden, nil
	}

	ctx := r.Context()
	fi, err := storage.Get(ctx, reqPath)
	if err != nil {
		return http.StatusNotFound, err
	}
	if fi.IsDir() {
		return http.StatusMethodNotAllowed, nil
	}

	etag := ""
	if e := fi.GetETag(); e != "" {
		etag = fmt.Sprintf(`"%s"`, e)
	}
	if etag != "" {
		w.Header().Set("ETag", etag)
	}

	body, size, err := storage.Open(ctx, reqPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer body.Close()

	w.Header().Set("Content-Type", fi.GetContentType())
	if fi.GetContentType() == "" {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Last-Modified", fi.ModTime().UTC().Format(http.TimeFormat))

	if r.Method == "HEAD" {
		return 0, nil
	}

	_, err = io.Copy(w, body)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request, storage Storage, user User) (status int, err error) {
	reqPath, status, err := h.stripPrefix(r.URL.Path)
	if err != nil {
		return status, err
	}
	if !user.CanRemove() {
		return http.StatusForbidden, nil
	}

	release, status, err := h.confirmLocks(r, reqPath, "")
	if err != nil {
		return status, err
	}
	defer release()

	ctx := r.Context()
	if err := storage.Remove(ctx, reqPath); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusNoContent, nil
}

func (h *Handler) handlePut(w http.ResponseWriter, r *http.Request, storage Storage, user User) (status int, err error) {
	reqPath, status, err := h.stripPrefix(r.URL.Path)
	if err != nil {
		return status, err
	}
	if !user.CanWrite() {
		return http.StatusForbidden, nil
	}

	release, status, err := h.confirmLocks(r, reqPath, "")
	if err != nil {
		return status, err
	}
	defer release()

	ctx := r.Context()
	size := r.ContentLength
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Check if exists
	_, err = storage.Get(ctx, reqPath)
	exists := err == nil

	if err := storage.Put(ctx, reqPath, r.Body, size, contentType); err != nil {
		return http.StatusInternalServerError, err
	}

	if exists {
		return http.StatusNoContent, nil
	}
	return http.StatusCreated, nil
}

func (h *Handler) handleMkcol(w http.ResponseWriter, r *http.Request, storage Storage, user User) (status int, err error) {
	reqPath, status, err := h.stripPrefix(r.URL.Path)
	if err != nil {
		return status, err
	}
	if !user.CanWrite() {
		return http.StatusForbidden, nil
	}

	release, status, err := h.confirmLocks(r, reqPath, "")
	if err != nil {
		return status, err
	}
	defer release()

	ctx := r.Context()
	if r.ContentLength > 0 {
		return http.StatusUnsupportedMediaType, nil
	}

	if _, err := storage.Get(ctx, reqPath); err == nil {
		return http.StatusMethodNotAllowed, nil
	}

	if err := storage.MakeDir(ctx, reqPath); err != nil {
		return http.StatusConflict, err
	}
	return http.StatusCreated, nil
}

func (h *Handler) handleCopyMove(w http.ResponseWriter, r *http.Request, storage Storage, user User) (status int, err error) {
	hdr := r.Header.Get("Destination")
	if hdr == "" {
		return http.StatusBadRequest, nil
	}

	u, err := url.Parse(hdr)
	if err != nil {
		return http.StatusBadRequest, err
	}
	if u.Host != "" && u.Host != r.Host {
		return http.StatusBadGateway, nil
	}

	src, status, err := h.stripPrefix(r.URL.Path)
	if err != nil {
		return status, err
	}
	dst, status, err := h.stripPrefix(u.Path)
	if err != nil {
		return status, err
	}

	if dst == "" {
		return http.StatusBadGateway, nil
	}
	if dst == src {
		return http.StatusForbidden, errDestinationEqualsSource
	}

	ctx := r.Context()

	if r.Method == "COPY" {
		if !user.CanCopy() {
			return http.StatusForbidden, nil
		}
	} else {
		if !user.CanMove() {
			return http.StatusForbidden, nil
		}
	}

	// Check Overwrite header
	overwrite := r.Header.Get("Overwrite")
	if overwrite == "" {
		overwrite = "T"
	}

	// Check if destination exists
	_, err = storage.Get(ctx, dst)
	dstExists := err == nil

	if dstExists {
		if overwrite == "F" {
			return http.StatusPreconditionFailed, nil
		}
		// Remove existing destination
		if err := storage.Remove(ctx, dst); err != nil {
			return http.StatusInternalServerError, err
		}
	}

	release, status, err := h.confirmLocks(r, src, dst)
	if err != nil {
		return status, err
	}
	defer release()

	if r.Method == "COPY" {
		if err := storage.Copy(ctx, src, dst); err != nil {
			return http.StatusInternalServerError, err
		}
	} else {
		if err := storage.Move(ctx, src, dst); err != nil {
			return http.StatusInternalServerError, err
		}
	}

	if dstExists {
		return http.StatusNoContent, nil
	}
	return http.StatusCreated, nil
}

func (h *Handler) handleLock(w http.ResponseWriter, r *http.Request, user User) (status int, err error) {
	reqPath, status, err := h.stripPrefix(r.URL.Path)
	if err != nil {
		return status, err
	}
	if !user.CanWebdavManage() {
		return http.StatusForbidden, nil
	}

	li, status, err := readLockInfo(r.Body)
	if err != nil {
		return status, err
	}

	ctx := r.Context()
	token, ld, now, created := "", LockDetails{}, time.Now(), false
	if li.XMLName.Local == "" {
		// Refresh lock
		ih, ok := parseIfHeader(r.Header.Get("If"))
		if !ok {
			return http.StatusBadRequest, nil
		}
		if len(ih.lists) != 1 || len(ih.lists[0].conditions) != 1 {
			return http.StatusBadRequest, nil
		}
		token = ih.lists[0].conditions[0].Token
		if token == "" {
			return http.StatusBadRequest, nil
		}
		timeout, err := parseTimeout(r.Header.Get("Timeout"))
		if err != nil {
			return http.StatusBadRequest, err
		}
		ld, err = h.LockSystem.Refresh(now, token, timeout)
		if err != nil {
			if err == ErrNoSuchLock {
				return http.StatusPreconditionFailed, err
			}
			return http.StatusInternalServerError, err
		}
	} else {
		// Create lock
		depth := infiniteDepth
		if hdr := r.Header.Get("Depth"); hdr != "" {
			depth = parseDepth(hdr)
			if depth != 0 && depth != infiniteDepth {
				return http.StatusBadRequest, nil
			}
		}
		timeout, err := parseTimeout(r.Header.Get("Timeout"))
		if err != nil {
			return http.StatusBadRequest, err
		}
		ld = LockDetails{
			Root:      reqPath,
			Duration:  timeout,
			OwnerXML:  li.Owner.InnerXML,
			ZeroDepth: depth == 0,
		}
		token, err = h.LockSystem.Create(now, ld)
		if err != nil {
			if err == ErrLocked {
				return StatusLocked, err
			}
			return http.StatusInternalServerError, err
		}
		defer func() {
			if status != 0 && status != http.StatusOK && status != http.StatusCreated {
				h.LockSystem.Unlock(now, token)
			}
		}()

		// Check if resource exists
		storage, ok := ctx.Value(storageKey).(Storage)
		if ok {
			_, err := storage.Get(ctx, reqPath)
			created = err != nil
		}
	}

	w.Header().Set("Lock-Token", "<"+token+">")
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	if created {
		w.WriteHeader(http.StatusCreated)
		writeLockInfo(w, token, ld)
		return 0, nil
	}
	w.WriteHeader(http.StatusOK)
	writeLockInfo(w, token, ld)
	return 0, nil
}

func (h *Handler) handleUnlock(w http.ResponseWriter, r *http.Request, user User) (status int, err error) {
	reqPath, status, err := h.stripPrefix(r.URL.Path)
	if err != nil {
		return status, err
	}
	_ = reqPath

	if !user.CanWebdavManage() {
		return http.StatusForbidden, nil
	}

	t := r.Header.Get("Lock-Token")
	if t == "" {
		return http.StatusBadRequest, nil
	}
	t = strings.TrimSuffix(strings.TrimPrefix(t, "<"), ">")

	switch err = h.LockSystem.Unlock(time.Now(), t); err {
	case nil:
		return http.StatusNoContent, nil
	case ErrForbidden:
		return http.StatusForbidden, err
	case ErrLocked:
		return StatusLocked, err
	case ErrNoSuchLock:
		return http.StatusConflict, err
	default:
		return http.StatusInternalServerError, err
	}
}

func (h *Handler) handlePropfind(w http.ResponseWriter, r *http.Request, storage Storage, user User) (status int, err error) {
	reqPath, status, err := h.stripPrefix(r.URL.Path)
	if err != nil {
		return status, err
	}
	if !user.CanWebdavRead() {
		return http.StatusForbidden, nil
	}

	ctx := r.Context()

	fi, err := storage.Get(ctx, reqPath)
	if err != nil {
		return http.StatusNotFound, err
	}

	depth := infiniteDepth
	if hdr := r.Header.Get("Depth"); hdr != "" {
		depth = parseDepth(hdr)
		if depth == invalidDepth {
			return http.StatusBadRequest, nil
		}
	}

	// 限制最大深度以防止超时
	if depth == infiniteDepth {
		depth = 1 // 默认限制为深度1，避免递归遍历整个目录树
	}

	pf, status, err := readPropfind(r.Body)
	if err != nil {
		return status, err
	}

	mw := multistatusWriter{w: w}
	walkFn := func(reqPath string, info FileInfo, err error) error {
		if err != nil {
			return err
		}
		var pstats []Propstat
		if pf.Propname != nil {
			pnames, err := propnames(ctx, h.LockSystem, info)
			if err != nil {
				return err
			}
			pstat := Propstat{Status: http.StatusOK}
			for _, pn := range pnames {
				pstat.Props = append(pstat.Props, Property{XMLName: pn})
			}
			pstats = append(pstats, pstat)
		} else if pf.Allprop != nil {
			pstats, err = allprop(ctx, h.LockSystem, info, pf.Include)
		} else {
			pstats, err = props(ctx, h.LockSystem, info, pf.Prop)
		}
		if err != nil {
			return err
		}
		href := (&url.URL{Path: h.Prefix + reqPath}).EscapedPath()
		if info.IsDir() && !strings.HasSuffix(href, "/") {
			href += "/"
		}
		return mw.write(makePropstatResponse(href, pstats))
	}

	walkErr := h.walkFS(ctx, storage, depth, reqPath, fi, walkFn)
	closeErr := mw.close()
	if walkErr != nil {
		return http.StatusInternalServerError, walkErr
	}
	if closeErr != nil {
		return http.StatusInternalServerError, closeErr
	}
	return 0, nil
}

func (h *Handler) handleProppatch(w http.ResponseWriter, r *http.Request, storage Storage, user User) (status int, err error) {
	reqPath, status, err := h.stripPrefix(r.URL.Path)
	if err != nil {
		return status, err
	}
	if !user.CanWebdavManage() {
		return http.StatusForbidden, nil
	}

	release, status, err := h.confirmLocks(r, reqPath, "")
	if err != nil {
		return status, err
	}
	defer release()

	ctx := r.Context()
	if _, err := storage.Get(ctx, reqPath); err != nil {
		return http.StatusNotFound, err
	}

	patches, status, err := readProppatch(r.Body)
	if err != nil {
		return status, err
	}
	pstats, err := patch(ctx, h.LockSystem, reqPath, patches)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	mw := multistatusWriter{w: w}
	href := (&url.URL{Path: h.Prefix + reqPath}).EscapedPath()
	writeErr := mw.write(makePropstatResponse(href, pstats))
	closeErr := mw.close()
	if writeErr != nil {
		return http.StatusInternalServerError, writeErr
	}
	if closeErr != nil {
		return http.StatusInternalServerError, closeErr
	}
	return 0, nil
}

func (h *Handler) walkFS(ctx context.Context, storage Storage, depth int, name string, info FileInfo, walkFn func(string, FileInfo, error) error) error {
	if err := walkFn(name, info, nil); err != nil {
		return err
	}
	if depth == 0 || !info.IsDir() {
		return nil
	}
	if depth == 1 {
		depth = 0
	}

	children, err := storage.List(ctx, name)
	if err != nil {
		return walkFn(name, info, err)
	}

	for _, child := range children {
		childPath := path.Join(name, child.GetName())
		if err := h.walkFS(ctx, storage, depth, childPath, child, walkFn); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) confirmLocks(r *http.Request, src, dst string) (func(), int, error) {
	hdr := r.Header.Get("If")
	if hdr == "" {
		// No lock confirmation required
		return func() {}, 0, nil
	}
	ih, ok := parseIfHeader(hdr)
	if !ok {
		return nil, http.StatusBadRequest, nil
	}
	for _, l := range ih.lists {
		lsrc := l.resourceTag
		if lsrc == "" {
			lsrc = src
		} else {
			u, err := url.Parse(lsrc)
			if err != nil {
				continue
			}
			lsrc, _, _ = h.stripPrefix(u.Path)
		}
		release, err := h.LockSystem.Confirm(time.Now(), lsrc, dst, l.conditions...)
		if err == ErrConfirmationFailed {
			continue
		}
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		return release, 0, nil
	}
	return nil, StatusLocked, ErrLocked
}

func makePropstatResponse(href string, pstats []Propstat) *response {
	resp := response{
		Href:     []string{href},
		Propstat: make([]propstat, len(pstats)),
	}
	for i, p := range pstats {
		var xmlErr *xmlError
		if p.XMLError != "" {
			xmlErr = &xmlError{InnerXML: []byte(p.XMLError)}
		}
		resp.Propstat[i] = propstat{
			Status:              fmt.Sprintf("HTTP/1.1 %d %s", p.Status, StatusText(p.Status)),
			Prop:                p.Props,
			ResponseDescription: p.ResponseDescription,
			Error:               xmlErr,
		}
	}
	return &resp
}

const (
	infiniteDepth = -1
	invalidDepth  = -2
)

func parseDepth(hdr string) int {
	switch hdr {
	case "0":
		return 0
	case "1":
		return 1
	case "infinity":
		return infiniteDepth
	}
	return invalidDepth
}
