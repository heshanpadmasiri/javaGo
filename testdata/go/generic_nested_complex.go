package converted

type complex struct {
	mapOfOpt       map[string]Optional[int]
	optOfMap       Optional[map[string][]int]
	listOfMaps     []map[int]string
	hashMapOfLists map[string][]bool
}

func newComplex() complex {
	this := complex{}
	return this
}
