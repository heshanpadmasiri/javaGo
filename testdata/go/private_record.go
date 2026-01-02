package converted

type privatePoint struct {
	X int
	Y int
}

func newPrivatePoint() PrivatePoint {
	this := PrivatePoint{}
	return this
}
