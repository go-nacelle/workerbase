package workerbase

type healthToken string

func (t healthToken) String() string {
	return "worker-init"
}
