package converted

type nested struct {
	listOfOpt     []Optional[string]
	optOfList     Optional[[]int]
	listOfResults []Result[bool]
}

func newNested() nested {
	this := nested{}
	return this
}
