package converted

type wildcards struct {
	rawList         []any
	rawOpt          Optional[any]
	mapWithWildcard map[string]any
	pairOfWildcards Pair[any, any]
}

func newWildcards() wildcards {
	this := wildcards{}
	return this
}
