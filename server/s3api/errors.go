package s3api

import (
	"encoding/xml"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// S3Error S3 错误定义
type S3Error struct {
	Code       string
	Message    string
	StatusCode int
}

// S3ErrorResponse S3 XML 错误响应
type S3ErrorResponse struct {
	XMLName   xml.Name `xml:"Error"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	Resource  string   `xml:"Resource,omitempty"`
	RequestID string   `xml:"RequestId"`
}

// S3 标准错误码
var (
	ErrAccessDenied = S3Error{
		Code:       "AccessDenied",
		Message:    "Access Denied",
		StatusCode: http.StatusForbidden,
	}
	ErrNoSuchBucket = S3Error{
		Code:       "NoSuchBucket",
		Message:    "The specified bucket does not exist",
		StatusCode: http.StatusNotFound,
	}
	ErrNoSuchKey = S3Error{
		Code:       "NoSuchKey",
		Message:    "The specified key does not exist",
		StatusCode: http.StatusNotFound,
	}
	ErrInvalidAccessKeyId = S3Error{
		Code:       "InvalidAccessKeyId",
		Message:    "The AWS Access Key Id you provided does not exist in our records",
		StatusCode: http.StatusForbidden,
	}
	ErrSignatureDoesNotMatch = S3Error{
		Code:       "SignatureDoesNotMatch",
		Message:    "The request signature we calculated does not match the signature you provided",
		StatusCode: http.StatusForbidden,
	}
	ErrInternalError = S3Error{
		Code:       "InternalError",
		Message:    "We encountered an internal error. Please try again.",
		StatusCode: http.StatusInternalServerError,
	}
	ErrMalformedXML = S3Error{
		Code:       "MalformedXML",
		Message:    "The XML you provided was not well-formed",
		StatusCode: http.StatusBadRequest,
	}
	ErrInvalidPart = S3Error{
		Code:       "InvalidPart",
		Message:    "One or more of the specified parts could not be found",
		StatusCode: http.StatusBadRequest,
	}
	ErrNoSuchUpload = S3Error{
		Code:       "NoSuchUpload",
		Message:    "The specified multipart upload does not exist",
		StatusCode: http.StatusNotFound,
	}
	ErrInvalidRequest = S3Error{
		Code:       "InvalidRequest",
		Message:    "Invalid Request",
		StatusCode: http.StatusBadRequest,
	}
	ErrMissingContentLength = S3Error{
		Code:       "MissingContentLength",
		Message:    "You must provide the Content-Length HTTP header",
		StatusCode: http.StatusLengthRequired,
	}
	ErrInvalidPartNumber = S3Error{
		Code:       "InvalidPartNumber",
		Message:    "The part number must be an integer between 1 and 10000",
		StatusCode: http.StatusBadRequest,
	}
	ErrEntityTooLarge = S3Error{
		Code:       "EntityTooLarge",
		Message:    "Your proposed upload exceeds the maximum allowed size",
		StatusCode: http.StatusBadRequest,
	}
	ErrBucketAlreadyExists = S3Error{
		Code:       "BucketAlreadyExists",
		Message:    "The requested bucket name is not available",
		StatusCode: http.StatusConflict,
	}
	ErrMethodNotAllowed = S3Error{
		Code:       "MethodNotAllowed",
		Message:    "The specified method is not allowed against this resource",
		StatusCode: http.StatusMethodNotAllowed,
	}
)

// WriteS3Error 写入 S3 错误响应
func WriteS3Error(c *gin.Context, err S3Error) {
	response := S3ErrorResponse{
		Code:      err.Code,
		Message:   err.Message,
		Resource:  c.Request.URL.Path,
		RequestID: generateRequestID(),
	}

	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.Status(err.StatusCode)

	xmlData, marshalErr := xml.MarshalIndent(response, "", "  ")
	if marshalErr != nil {
		c.Status(500)
		return
	}

	c.Writer.Write([]byte(xml.Header))
	c.Writer.Write(xmlData)
}

// WriteS3ErrorWithMessage 写入带自定义消息的 S3 错误响应
func WriteS3ErrorWithMessage(c *gin.Context, err S3Error, message string) {
	response := S3ErrorResponse{
		Code:      err.Code,
		Message:   message,
		Resource:  c.Request.URL.Path,
		RequestID: generateRequestID(),
	}

	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.Status(err.StatusCode)

	xmlData, marshalErr := xml.MarshalIndent(response, "", "  ")
	if marshalErr != nil {
		c.Status(500)
		return
	}

	c.Writer.Write([]byte(xml.Header))
	c.Writer.Write(xmlData)
}

// generateRequestID 生成请求 ID
func generateRequestID() string {
	return uuid.New().String()
}
