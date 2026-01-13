package converted

type deep struct {
	deep        Optional[Optional[Optional[string]]]
	multiNested map[string]map[int][]string
	veryComplex []Optional[map[string]Result[int]]
}

func newDeep() deep {
	this := deep{}
	return this
}
