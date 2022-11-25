package v2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"mime/multipart"
	"net/http"
)

type (
	Requester[T any, E any] struct {
		client *Client
	}

	requestQuery struct {
		Query     Query          `json:"query"`
		Variables QueryVariables `json:"variables"`
	}

	Response[T any, E any] struct {
		Data   T               `json:"data"`
		Errors []GraphError[E] `json:"errors"`
	}
)

func NewRequester[T any, E any](client *Client) *Requester[T, E] {
	return &Requester[T, E]{
		client: client,
	}
}

func (r *Requester[T, E]) Request(ctx context.Context, req *Request) (Response[T, E], error) {
	var (
		res Response[T, E]
	)
	if len(req.files) > 0 && !r.client.useMultipartForm {
		return res, ErrInvalidInput
	}

	if r.client.useMultipartForm {
		return r.requestMultipart(ctx, req)
	}

	return r.requestJSON(ctx, req)
}

func (r *Requester[T, E]) requestMultipart(ctx context.Context, req *Request) (Response[T, E], error) {
	var (
		request  bytes.Buffer
		httpReq  *http.Request
		httpRes  *http.Response
		response Response[T, E]
		err      error
	)
	writer := multipart.NewWriter(&request)
	if err = writer.WriteField("query", req.q.String()); err != nil {
		return response, err
	}
	if len(req.vars) > 0 {
		var (
			variablesField io.Writer
			variablesBuff  bytes.Buffer
		)
		if variablesField, err = writer.CreateFormField("variables"); err != nil {
			return response, NewError(err, ErrCreateVariablesField)
		}
		if err = json.NewEncoder(io.MultiWriter(variablesField, &variablesBuff)).Encode(req.vars); err != nil {
			return response, NewError(err, ErrEncodeVariablesField)
		}
	}
	for i := range req.files {
		part, err := writer.CreateFormFile(req.files[i].Field, req.files[i].Name)
		if err != nil {
			return response, NewError(err, ErrCreateFile)
		}
		if _, err = io.Copy(part, req.files[i].R); err != nil {
			return response, NewError(err, ErrCopy)
		}
	}
	if err = writer.Close(); err != nil {
		return response, errors.Wrap(err, "close writer")
	}

	if httpReq, err = http.NewRequestWithContext(ctx, http.MethodPost, r.client.endpoint, &request); err != nil {
		return response, err
	}

	r.setRequestHeaders(httpReq, req, writer.FormDataContentType())

	if httpRes, err = r.client.httpClient.Do(httpReq); err != nil {
		return response, err
	}
	defer httpRes.Body.Close()

	body, err := io.ReadAll(httpRes.Body)
	if err != nil {
		if httpRes.StatusCode != http.StatusOK {
			return response, fmt.Errorf("%v: %v", ErrRequest, httpRes.StatusCode)
		}
		return response, NewError(err, ErrReadBody)
	}

	if err = json.Unmarshal(body, &response); err != nil {
		return response, NewError(err, ErrDecode)
	}

	return response, nil
}

func (r *Requester[T, E]) setRequestHeaders(httpReq *http.Request, req *Request, contentType string) {
	httpReq.Close = r.client.closeReq
	httpReq.Header.Set("Content-Type", contentType)
	httpReq.Header.Set("Accept", "application/json; charset=utf-8")
	for key, values := range req.Header {
		for i := range values {
			httpReq.Header.Add(key, values[i])
		}
	}
}

func (r *Requester[T, E]) requestJSON(ctx context.Context, req *Request) (Response[T, E], error) {
	var (
		httpReq  *http.Request
		httpRes  *http.Response
		request  bytes.Buffer
		response Response[T, E]
		err      error
	)
	if err = json.NewEncoder(&request).Encode(requestQuery{Query: req.q, Variables: req.vars}); err != nil {
		return response, err
	}
	if httpReq, err = http.NewRequestWithContext(ctx, http.MethodPost, r.client.endpoint, &request); err != nil {
		return response, err
	}

	r.setRequestHeaders(httpReq, req, "application/json; charset=utf-8")

	if httpRes, err = r.client.httpClient.Do(httpReq.WithContext(ctx)); err != nil {
		return response, err
	}
	defer httpRes.Body.Close()

	body, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return response, NewError(err, ErrReadBody)
	}

	if err = json.Unmarshal(body, &response); err != nil {
		if httpRes.StatusCode != http.StatusOK {
			return response, fmt.Errorf("%v: %v", ErrRequest, httpRes.StatusCode)
		}
		return response, errors.Wrap(err, "decoding response")
	}

	return response, nil
}
