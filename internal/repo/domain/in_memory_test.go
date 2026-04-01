package domain

import (
	"testing"

	"github.com/ericls/certmatic/internal/repo/repotest"
	pkgdomain "github.com/ericls/certmatic/pkg/domain"
)

func TestInMemoryDomainRepo(t *testing.T) {
	repotest.RunDomainRepoTests(t, func(t *testing.T) pkgdomain.DomainRepo {
		return NewInMemoryDomainRepo("test-memory")
	})
}
