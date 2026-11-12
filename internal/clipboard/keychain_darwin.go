//go:build darwin

package clipboard

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ebitengine/purego"
)

// KeychainService is the Keychain service of the history's key.
const KeychainService = "dev.gocast.app.clipboard"

const keychainAccount = "history-key"

// Keychain keeps the history's key in the user's login Keychain, as a
// generic password (Security.framework's SecItem calls).
type Keychain struct {
	Service, Account string
}

// NewKeychain is the Keychain item the app keeps its key in.
func NewKeychain() *Keychain {
	return &Keychain{Service: KeychainService, Account: keychainAccount}
}

// KeychainError is a Security.framework status (OSStatus).
type KeychainError int32

func (e KeychainError) Error() string {
	switch e {
	case -128:
		return "keychain: the user canceled access (errSecUserCanceled)"
	case -25293:
		return "keychain: access denied (errSecAuthFailed)"
	case -25308:
		return "keychain: interaction not allowed, the keychain may be locked (errSecInteractionNotAllowed)"
	case -34018:
		return "keychain: missing entitlement (errSecMissingEntitlement)"
	}
	return fmt.Sprintf("keychain: OSStatus %d", int32(e))
}

const (
	errSecItemNotFound = -25300
	errSecSuccess      = 0
)

var sec struct {
	once        sync.Once
	ok          bool
	add         func(attrs uintptr, result *uintptr) int32
	copyMatch   func(query uintptr, result *uintptr) int32
	del         func(query uintptr) int32
	class       uintptr
	genericPass uintptr
	service     uintptr
	account     uintptr
	label       uintptr
	valueData   uintptr
	returnData  uintptr
	matchLimit  uintptr
	limitOne    uintptr
}

func loadSecurity() bool {
	sec.once.Do(func() {
		if !loadCF() || libs.security == 0 {
			return
		}
		lib := libs.security
		purego.RegisterLibFunc(&sec.add, lib, "SecItemAdd")
		purego.RegisterLibFunc(&sec.copyMatch, lib, "SecItemCopyMatching")
		purego.RegisterLibFunc(&sec.del, lib, "SecItemDelete")
		sec.class = symbol(lib, "kSecClass")
		sec.genericPass = symbol(lib, "kSecClassGenericPassword")
		sec.service = symbol(lib, "kSecAttrService")
		sec.account = symbol(lib, "kSecAttrAccount")
		sec.label = symbol(lib, "kSecAttrLabel")
		sec.valueData = symbol(lib, "kSecValueData")
		sec.returnData = symbol(lib, "kSecReturnData")
		sec.matchLimit = symbol(lib, "kSecMatchLimit")
		sec.limitOne = symbol(lib, "kSecMatchLimitOne")
		sec.ok = sec.class != 0 && sec.genericPass != 0 && sec.service != 0 && sec.account != 0 &&
			sec.label != 0 && sec.valueData != 0 && sec.returnData != 0 && sec.matchLimit != 0 && sec.limitOne != 0
	})
	return sec.ok
}

var errNoSecurity = errors.New("keychain: Security.framework unavailable")

// Load reads the key; ErrNoKey when there is none.
func (k *Keychain) Load() ([]byte, error) {
	if !loadSecurity() {
		return nil, errNoSecurity
	}
	var refs cfRefs
	defer refs.release()
	q := refs.dict(
		sec.class, sec.genericPass,
		sec.service, refs.str(k.Service),
		sec.account, refs.str(k.Account),
		sec.returnData, cf.booleanTrue,
		sec.matchLimit, sec.limitOne,
	)
	var result uintptr
	switch status := sec.copyMatch(q, &result); status {
	case errSecSuccess:
	case errSecItemNotFound:
		return nil, ErrNoKey
	default:
		return nil, KeychainError(status)
	}
	if result == 0 {
		return nil, ErrNoKey
	}
	defer cf.release(result)
	return cfBytes(result), nil
}

// Save stores key, replacing an earlier one.
func (k *Keychain) Save(key []byte) error {
	if !loadSecurity() {
		return errNoSecurity
	}
	if err := k.Delete(); err != nil {
		return err
	}
	var refs cfRefs
	defer refs.release()
	attrs := refs.dict(
		sec.class, sec.genericPass,
		sec.service, refs.str(k.Service),
		sec.account, refs.str(k.Account),
		sec.label, refs.str("Gocast Clipboard History"),
		sec.valueData, refs.data(key),
	)
	if status := sec.add(attrs, nil); status != errSecSuccess {
		return KeychainError(status)
	}
	return nil
}

// Delete removes the key, if there is one.
func (k *Keychain) Delete() error {
	if !loadSecurity() {
		return errNoSecurity
	}
	var refs cfRefs
	defer refs.release()
	q := refs.dict(
		sec.class, sec.genericPass,
		sec.service, refs.str(k.Service),
		sec.account, refs.str(k.Account),
	)
	if status := sec.del(q); status != errSecSuccess && status != errSecItemNotFound {
		return KeychainError(status)
	}
	return nil
}
