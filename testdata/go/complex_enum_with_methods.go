package converted

type Status struct {
	value string
}

var Status_ACTIVE = Status{value: "active"}
var Status_INACTIVE = Status{value: "inactive"}

func FromString(s string) Status {
	// migrated from complex_enum_with_methods.java:15:5
	if "active" == s {
		return Status_ACTIVE
	}
	return Status_INACTIVE
}

func (this *Status) GetValue() string {
	// migrated from complex_enum_with_methods.java:11:5
	return this.value
}
