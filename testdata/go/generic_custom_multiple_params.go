package converted

type pairs struct {
	pair   Tuple[string, int]
	triple Triple[string, int, bool]
	either Either[int64, string]
}

func newPairs() pairs {
	this := pairs{}
	return this
}
