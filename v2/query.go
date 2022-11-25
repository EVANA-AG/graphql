package v2

type (
	Query string
)

func (q Query) String() string {
	return string(q)
}
