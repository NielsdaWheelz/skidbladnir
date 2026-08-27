package platform

type Kind string

const (
	KindLinux  Kind = "Linux"
	KindDarwin Kind = "Darwin"
)

type Descriptor struct {
	Kind Kind
}

func Current() Descriptor { return current() }
