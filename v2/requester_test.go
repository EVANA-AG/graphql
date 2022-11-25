package graphql_next

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type (
	testData struct {
		Something string `json:"something"`
	}

	testErrorExtension struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	}
	testExtendedError GraphError[testErrorExtension]

	testResponse struct {
		Data   testData            `json:"data"`
		Errors []testExtendedError `json:"errors"`
	}
)

func noop(w http.ResponseWriter, r *http.Request) {}

func createMiddleware(handler http.HandlerFunc) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			handler(w, r)
			next.ServeHTTP(w, r)
		}

		return http.HandlerFunc(fn)
	}
}

func createHTTPTestServer[T any](t *testing.T, calls *int, requestBodyData string, response T, handler http.HandlerFunc) *httptest.Server {
	fn := func(w http.ResponseWriter, r *http.Request) {
		*calls++
		assert.Equal(t, r.Method, http.MethodPost)
		b, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, string(b), requestBodyData)
		b, err = json.Marshal(response)
		assert.NoError(t, err)
		_, err = w.Write(b)
		assert.NoError(t, err)
	}
	middleware := createMiddleware(handler)

	return httptest.NewServer(middleware(http.HandlerFunc(fn)))
}

func TestNewRequester(t *testing.T) {
	requestData := `{"query":"query {}","variables":null}` + "\n"

	t.Run("should successfully response with data", func(t *testing.T) {
		var calls int
		responseData := testResponse{Data: testData{Something: "yes"}}
		server := createHTTPTestServer[testResponse](t, &calls, requestData, responseData, noop)

		defer server.Close()
		ctx := context.Background()
		client := NewClient(server.URL)
		requester := NewRequester[testData, any](client)

		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
		response, err := requester.Request(ctx, &Request{q: "query {}"})
		assert.NoError(t, err)
		assert.Equal(t, calls, 1)
		fmt.Println(response.Data)
		assert.Equal(t, response.Data, responseData.Data)
	})

	t.Run("should return an error in response with extension", func(t *testing.T) {
		var calls int
		expectedError := testErrorExtension{
			Message: "extension error",
			Code:    http.StatusBadRequest,
		}
		expectedResponse := testResponse{
			Errors: []testExtendedError{
				{
					Message:    "miscellaneous message as to why the the request was bad",
					Extensions: expectedError,
				},
			},
		}

		server := createHTTPTestServer[testResponse](t, &calls, requestData, expectedResponse, noop)
		defer server.Close()

		ctx := context.Background()
		client := NewClient(server.URL)
		requester := NewRequester[testData, testErrorExtension](client)

		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)

		defer cancel()
		response, err := requester.Request(ctx, &Request{q: "query {}"})
		assert.NoError(t, err)
		assert.Len(t, response.Errors, 1)
		graphqlError := response.Errors[0]
		assert.Equal(t, graphqlError.GetExtensions().Code, expectedError.Code)
		assert.Equal(t, graphqlError.GetExtensions().Message, expectedError.Message)
		assert.Equal(t, calls, 1)
	})

	t.Run("should successfully perform query with variables", func(t *testing.T) {
		var calls int
		username := "testUser"
		query := fmt.Sprintf(`{"query":"query {}","variables":{"username":"%s"}}`, username) + "\n"

		expectedData := testData{Something: "some data"}
		expectedResponse := testResponse{Data: expectedData}

		server := createHTTPTestServer[testResponse](t, &calls, query, expectedResponse, noop)
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		request := NewRequest("query {}")
		assert.NotNil(t, request)
		request.Var("username", username)
		assert.Equal(t, request.vars["username"], username)

		client := NewClient(server.URL)
		requester := NewRequester[testData, any](client)
		response, err := requester.Request(ctx, request)
		assert.NoError(t, err)
		assert.Equal(t, expectedData.Something, response.Data.Something)
		assert.Equal(t, calls, 1)
	})

	t.Run("should return and error, when the server has status > 200", func(t *testing.T) {
		var calls int
		response := testResponse{}
		errorMessage := "Internal Server error"
		server := createHTTPTestServer[testResponse](t, &calls, requestData, response, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte(errorMessage))
			assert.NoError(t, err)
		})
		defer server.Close()

		client := NewClient(server.URL)
		requester := NewRequester[testData, any](client)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		request := NewRequest("query {}")
		_, err := requester.Request(ctx, request)
		assert.Error(t, err)
		assert.Equal(t, err.Error(), fmt.Sprintf("%s: 500", ErrRequest))
		assert.Equal(t, calls, 1)
	})

	t.Run("should set headers for the graphql request", func(t *testing.T) {
		var calls int
		header := "Authorization"
		headerValue := "Bearer token"
		username := "testUser"
		query := fmt.Sprintf(`{"query":"query {}","variables":{"username":"%s"}}`, username) + "\n"
		expectedData := testData{Something: "some data"}
		expectedResponse := testResponse{Data: expectedData}

		server := createHTTPTestServer[testResponse](t, &calls, query, expectedResponse, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get(header), headerValue)
		})
		defer server.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		client := NewClient(server.URL)
		requester := NewRequester[testData, any](client)
		request := NewRequest("query {}")

		request.Header.Set(header, headerValue)
		request.Var("username", username)
		assert.Equal(t, request.vars["username"], username)
		assert.Equal(t, request.Header.Get(header), headerValue)

		response, err := requester.Request(ctx, request)

		assert.NoError(t, err)
		assert.Equal(t, calls, 1)
		assert.Equal(t, expectedData.Something, response.Data.Something)
	})
}
