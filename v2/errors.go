package graphql_next

import "github.com/pkg/errors"

var (
	ErrInvalidInput         = errors.New("cannot send files with PostFields option")
	ErrCreateVariablesField = errors.New("create variables field")
	ErrEncodeVariablesField = errors.New("encode variables")
	ErrCreateFile           = errors.New("create form file")
	ErrReadBody             = errors.New("read body")
	ErrDecode               = errors.New("decode")
	ErrCopy                 = errors.New("copy")
	ErrRequest              = errors.New("graphql: server returned a non-200 status code")
)

type (
	GraphqlError[T any] interface {
		Message() string
		Error() string
		Extensions() T
	}

	GraphError[T any] struct {
		Message    string `json:"message"`
		Extensions T      `json:"extensions"`
	}
)

func (e GraphError[T]) Error() string {
	return "graphql: " + e.Message
}

func (e GraphError[T]) GetExtensions() T {
	return e.Extensions
}

func NewError(err error, wrappedErr error) error {
	return errors.Wrap(err, wrappedErr.Error())
}
