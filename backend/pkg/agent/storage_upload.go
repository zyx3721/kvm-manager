package agent

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

func (c *Client) UploadStorageVolume(ctx context.Context, cfg Config, poolName string, volumeName string, fileName string, content io.Reader) (StorageVolume, error) {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	errCh := make(chan error, 1)
	go func() {
		defer writer.Close()
		if strings.TrimSpace(volumeName) != "" {
			if err := multipartWriter.WriteField("name", volumeName); err != nil {
				errCh <- err
				_ = writer.CloseWithError(err)
				return
			}
		}
		part, err := multipartWriter.CreateFormFile("file", fileName)
		if err != nil {
			errCh <- err
			_ = writer.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, content); err != nil {
			errCh <- err
			_ = writer.CloseWithError(err)
			return
		}
		if err := multipartWriter.Close(); err != nil {
			errCh <- err
			_ = writer.CloseWithError(err)
			return
		}
		errCh <- nil
	}()
	var volume StorageVolume
	if err := c.withTimeout(storageOperationTimeout).doReaderWithContentType(ctx, http.MethodPost, cfg, "/v1/storage-pools/"+urlPathEscape(poolName)+"/volumes/upload", reader, multipartWriter.FormDataContentType(), &volume); err != nil {
		_ = reader.CloseWithError(err)
		return StorageVolume{}, err
	}
	if err := <-errCh; err != nil {
		return StorageVolume{}, err
	}
	return volume, nil
}
