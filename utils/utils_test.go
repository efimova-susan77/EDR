package utils

import (
	"testing"
)

var (
	pid = 604
)

func TestRegQuery(t *testing.T) {
	/*path := `HKLM\System\CurrentControlSet\Services\SysmonDrv\Parameters\HashingAlgorithm`
	key, value := filepath.Split(path)
	t.Logf("Sysmon hashing algorithm: %s", RegQuery(key, value))*/
}

var (
	aclDirectories = []string{"C:\\Windows\\System32"}
)

func TestSetAuditACL(t *testing.T) {
	if err := SetEDRAuditACL(aclDirectories...); err != nil {
		t.Logf("Failed at setting Audit ACL: %s", err)
		t.FailNow()
	}
	t.Logf("Successfully set audit ACL")
}
func TestRemoveAuditACL(t *testing.T) {
	if err := RemoveEDRAuditACL(aclDirectories...); err != nil {
		t.Logf("Failed at setting Audit ACL: %s", err)
		t.FailNow()
	}
	t.Logf("Successfully set audit ACL")
}

func TestDisableFSAuditing(t *testing.T) {
	if err := DisableAuditPolicy("File System"); err != nil {
		t.Errorf("Failed at disabling FS Auditing: %s", err)
	}
}

func TestEnableFSAuditing(t *testing.T) {
	if err := EnableAuditPolicy("{0CCE921D-69AE-11D9-BED3-505054503030}"); err != nil {
		t.Errorf("Failed at enabling FS Auditing: %s", err)
	}
}
