package converted

type BaseData interface {
}

type Base interface {
	BaseData
}

type Type int

type BaseBase struct {
}

type BaseMethods struct {
	Self Base
}

const (
	Type_TYPE_A Type = iota
	Type_TYPE_B
)
