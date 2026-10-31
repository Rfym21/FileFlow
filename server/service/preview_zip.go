package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	zipPreviewBlockBytes   = 64 * 1024
	maxZIPMetadataBytes    = 8 * 1024 * 1024
	maxZIPDirectoryEntries = 10000
	maxZIPPreviewEntries   = 1000
)

type ZIPPreviewEntry struct {
	Name           string `json:"name"`
	Size           uint64 `json:"size"`
	CompressedSize uint64 `json:"compressedSize"`
	ModifiedAt     string `json:"modifiedAt,omitempty"`
	IsDir          bool   `json:"isDir"`
}

type zipPreviewDirectory struct {
	entries    []ZIPPreviewEntry
	entryCount int
	truncated  bool
}

// zipRangeReaderAt never reads or allocates an entire object. Every response must
// match the requested range, and cached metadata blocks have a fixed budget.
type zipRangeReaderAt struct {
	lock      sync.Mutex
	ctx       context.Context
	client    *s3.Client
	bucket    string
	key       string
	size      int64
	etag      *string
	blocks    map[int64][]byte
	readBytes int64
}

func (r *zipRangeReaderAt) ReadAt(p []byte, offset int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	if offset < 0 {
		return 0, fmt.Errorf("ZIP 目录偏移无效")
	}
	if offset >= r.size {
		return 0, io.EOF
	}
	available := min(int64(len(p)), r.size-offset)
	n := 0
	for int64(n) < available {
		position := offset + int64(n)
		blockStart := (position / zipPreviewBlockBytes) * zipPreviewBlockBytes
		block, err := r.block(blockStart)
		if err != nil {
			return n, err
		}
		inBlock := position - blockStart
		copied := copy(p[n:int(available)], block[inBlock:])
		n += copied
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (r *zipRangeReaderAt) block(start int64) ([]byte, error) {
	if block, ok := r.blocks[start]; ok {
		return block, nil
	}
	length := min(int64(zipPreviewBlockBytes), r.size-start)
	if length <= 0 || r.readBytes+length > maxZIPMetadataBytes {
		return nil, fmt.Errorf("ZIP 目录读取超过 8 MiB 限制，请下载后查看")
	}
	end := start + length - 1
	object, err := r.client.GetObject(r.ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket), Key: aws.String(r.key),
		Range: aws.String(fmt.Sprintf("bytes=%d-%d", start, end)), IfMatch: r.etag,
	})
	if err != nil {
		return nil, fmt.Errorf("读取 ZIP 目录失败: %w", err)
	}
	if object.Body == nil {
		return nil, fmt.Errorf("存储未返回 ZIP 目录内容")
	}
	defer object.Body.Close()
	expectedRange := fmt.Sprintf("bytes %d-%d/%d", start, end, r.size)
	if aws.ToString(object.ContentRange) != expectedRange {
		return nil, fmt.Errorf("存储未正确响应 ZIP 范围读取，请下载后查看")
	}
	block, err := io.ReadAll(io.LimitReader(object.Body, length+1))
	if err != nil {
		return nil, fmt.Errorf("读取 ZIP 目录失败: %w", err)
	}
	if int64(len(block)) != length {
		return nil, fmt.Errorf("ZIP 范围响应长度异常，请重新预览或下载")
	}
	r.readBytes += length
	r.blocks[start] = block
	return block, nil
}

type zipDirectoryEnd struct {
	count, size, offset uint64
	endOffset           int64
}

func readZIPDirectoryEnd(reader io.ReaderAt, size int64) (zipDirectoryEnd, error) {
	var result zipDirectoryEnd
	if size < 22 {
		return result, zip.ErrFormat
	}
	// Match archive/zip's footer search: first the last KiB, then the maximum
	// footer/comment window. A signature within an incomplete comment is invalid.
	for _, window := range []int64{1024, 65557} {
		window = min(window, size)
		tail := make([]byte, window)
		if _, err := reader.ReadAt(tail, size-window); err != nil {
			return result, err
		}
		for i := len(tail) - 22; i >= 0; i-- {
			if binary.LittleEndian.Uint32(tail[i:]) != 0x06054b50 {
				continue
			}
			if i+22+int(binary.LittleEndian.Uint16(tail[i+20:])) > len(tail) {
				break
			}
			footer := tail[i:]
			if binary.LittleEndian.Uint16(footer[4:]) != 0 || binary.LittleEndian.Uint16(footer[6:]) != 0 || binary.LittleEndian.Uint16(footer[8:]) != binary.LittleEndian.Uint16(footer[10:]) {
				return result, fmt.Errorf("暂不支持分卷 ZIP 预览")
			}
			result.count = uint64(binary.LittleEndian.Uint16(footer[10:]))
			result.size = uint64(binary.LittleEndian.Uint32(footer[12:]))
			result.offset = uint64(binary.LittleEndian.Uint32(footer[16:]))
			result.endOffset = size - window + int64(i)
			if result.count == 0xffff || result.size == 0xffffffff || result.offset == 0xffffffff {
				locator := make([]byte, 20)
				if result.endOffset < 20 {
					return result, zip.ErrFormat
				}
				if _, err := reader.ReadAt(locator, result.endOffset-20); err != nil {
					return result, err
				}
				if binary.LittleEndian.Uint32(locator) == 0x07064b50 {
					if binary.LittleEndian.Uint32(locator[4:]) != 0 || binary.LittleEndian.Uint32(locator[16:]) != 1 {
						return result, fmt.Errorf("暂不支持分卷 ZIP64 预览")
					}
					zip64Offset := binary.LittleEndian.Uint64(locator[8:])
					if size < 56 || zip64Offset > uint64(size-56) {
						return result, zip.ErrFormat
					}
					footer64 := make([]byte, 56)
					if _, err := reader.ReadAt(footer64, int64(zip64Offset)); err != nil {
						return result, err
					}
					if binary.LittleEndian.Uint32(footer64) != 0x06064b50 || binary.LittleEndian.Uint64(footer64[4:]) < 44 {
						return result, zip.ErrFormat
					}
					if binary.LittleEndian.Uint32(footer64[16:]) != 0 || binary.LittleEndian.Uint32(footer64[20:]) != 0 || binary.LittleEndian.Uint64(footer64[24:]) != binary.LittleEndian.Uint64(footer64[32:]) {
						return result, fmt.Errorf("暂不支持分卷 ZIP64 预览")
					}
					result.count = binary.LittleEndian.Uint64(footer64[32:])
					result.size = binary.LittleEndian.Uint64(footer64[40:])
					result.offset = binary.LittleEndian.Uint64(footer64[48:])
					result.endOffset = int64(zip64Offset)
				}
			}
			return result, nil
		}
		if window == size {
			break
		}
	}
	return result, zip.ErrFormat
}

func parseZIPPreviewDirectory(directory []byte, expectedCount uint64) (*zip.Reader, int, error) {
	// Validate the actual count before archive/zip allocates File objects. A false
	// footer count must not bypass the entry bound or extend parsing into payloads.
	count := 0
	for offset := 0; offset < len(directory); {
		remaining := directory[offset:]
		if len(remaining) >= 6 && binary.LittleEndian.Uint32(remaining) == 0x05054b50 && len(remaining) == 6+int(binary.LittleEndian.Uint16(remaining[4:])) {
			break // optional central-directory digital signature
		}
		if len(remaining) < 46 || binary.LittleEndian.Uint32(remaining) != 0x02014b50 {
			return nil, 0, zip.ErrFormat
		}
		length := 46 + int(binary.LittleEndian.Uint16(remaining[28:])) + int(binary.LittleEndian.Uint16(remaining[30:])) + int(binary.LittleEndian.Uint16(remaining[32:]))
		if length > len(remaining) {
			return nil, 0, zip.ErrFormat
		}
		count++
		if count > maxZIPDirectoryEntries {
			return nil, 0, fmt.Errorf("ZIP 目录超过 %d 个条目，请下载后查看", maxZIPDirectoryEntries)
		}
		offset += length
	}
	if uint64(count) != expectedCount {
		return nil, 0, fmt.Errorf("ZIP 目录条目数量不一致，文件可能已损坏")
	}
	// Parse only an isolated copy of the bounded central directory. No local file
	// headers or file contents are needed, and ZIP64 entry metadata stays intact.
	footer := make([]byte, 22)
	binary.LittleEndian.PutUint32(footer, 0x06054b50)
	binary.LittleEndian.PutUint16(footer[8:], uint16(count))
	binary.LittleEndian.PutUint16(footer[10:], uint16(count))
	binary.LittleEndian.PutUint32(footer[12:], uint32(len(directory)))
	archiveMetadata := append(directory, footer...)
	archive, err := zip.NewReader(bytes.NewReader(archiveMetadata), int64(len(archiveMetadata)))
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return nil, 0, fmt.Errorf("无法解析 ZIP 目录: %w", err)
	}
	if archive == nil || len(archive.File) != count {
		return nil, 0, zip.ErrFormat
	}
	return archive, count, nil
}

func readZIPPreview(ctx context.Context, client *s3.Client, bucket, key string, metadata *s3.HeadObjectOutput) (zipPreviewDirectory, error) {
	var result zipPreviewDirectory
	size := aws.ToInt64(metadata.ContentLength)
	reader := &zipRangeReaderAt{ctx: ctx, client: client, bucket: bucket, key: key, size: size, etag: metadata.ETag, blocks: make(map[int64][]byte)}
	end, err := readZIPDirectoryEnd(reader, size)
	if err != nil {
		return result, fmt.Errorf("无法读取 ZIP 目录: %w", err)
	}
	if end.count <= uint64(^uint(0)>>1) {
		result.entryCount = int(end.count)
	}
	if end.count > maxZIPDirectoryEntries {
		return result, fmt.Errorf("ZIP 目录超过 %d 个条目，请下载后查看", maxZIPDirectoryEntries)
	}
	if end.size > maxZIPMetadataBytes {
		return result, fmt.Errorf("ZIP 目录超过 8 MiB，请下载后查看")
	}
	if end.size > uint64(end.endOffset) || end.offset > uint64(size) {
		return result, zip.ErrFormat
	}
	directoryStart := end.endOffset - int64(end.size)
	if int64(end.size) > size-directoryStart {
		return result, zip.ErrFormat
	}
	directory := make([]byte, int(end.size))
	var archive *zip.Reader
	var count int
	// A padding archive may use the original offset. SFX prefixes can also contain
	// a decoy signature there, so only prefer a fully validated central directory.
	if uint64(directoryStart) > end.offset && end.count > 0 && end.size >= 46 && end.size <= uint64(size)-end.offset {
		header := make([]byte, 4)
		if _, err := reader.ReadAt(header, int64(end.offset)); err != nil {
			return result, err
		}
		if binary.LittleEndian.Uint32(header) == 0x02014b50 {
			if _, err := reader.ReadAt(directory, int64(end.offset)); err != nil {
				// Transport, changed-object and budget errors still abort the preview.
				return result, err
			}
			candidate, candidateCount, candidateErr := parseZIPPreviewDirectory(directory, end.count)
			if candidateErr == nil {
				archive, count = candidate, candidateCount
			}
		}
	}
	if archive == nil {
		if _, err := reader.ReadAt(directory, directoryStart); err != nil {
			return result, err
		}
		archive, count, err = parseZIPPreviewDirectory(directory, end.count)
		if err != nil {
			return result, err
		}
	}
	result.entries = make([]ZIPPreviewEntry, 0, min(count, maxZIPPreviewEntries))
	result.truncated = count > maxZIPPreviewEntries
	for _, file := range archive.File[:min(count, maxZIPPreviewEntries)] {
		entry := ZIPPreviewEntry{Name: file.Name, Size: file.UncompressedSize64, CompressedSize: file.CompressedSize64, IsDir: file.FileInfo().IsDir()}
		if !file.Modified.IsZero() {
			entry.ModifiedAt = file.Modified.UTC().Format(time.RFC3339)
		}
		result.entries = append(result.entries, entry)
	}
	return result, nil
}
