package converted

type container struct {
	stringOpt  Optional[string]
	intResult  Result[int]
	futureBool Future[bool]
}

func newContainer() container {
	this := container{}
	return this
}
