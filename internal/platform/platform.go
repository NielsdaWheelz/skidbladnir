package platform

type Kind string

const (
	KindLinux  Kind = "Linux"
	KindDarwin Kind = "Darwin"
)

type Descriptor struct {
	Kind                Kind
	TmuxPath            string
	TmuxVersion         string
	CodexNodeEntrypoint string
}

func Current() Descriptor { return current() }
