//go:build !windows

package winutil

func RegistryRead(encoded string) (any, uint32, error) {
	return nil, 0, ErrUnsupported
}

func RegistryWrite(encoded string, value any) error {
	return ErrUnsupported
}

func RegistryDelete(encoded string) error {
	return ErrUnsupported
}

func ParseHexString(s string) ([]byte, error) {
	return nil, ErrUnsupported
}
