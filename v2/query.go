package graphql

type (
	Query string
)

func (q Query) String() string {
	return string(q)
}
