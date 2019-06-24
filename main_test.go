package workerbase

//go:generate go-mockgen -f github.com/go-nacelle/workerbase -i workerSpecFinalizer -o worker_spec_mock_test.go

import (
	"testing"

	"github.com/aphistic/sweet"
	junit "github.com/aphistic/sweet-junit"
	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {
	RegisterFailHandler(sweet.GomegaFail)

	sweet.Run(m, func(s *sweet.S) {
		s.RegisterPlugin(junit.NewPlugin())

		s.AddSuite(&WorkerSuite{})
	})
}
