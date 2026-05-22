package pointer

import "github.com/google/uuid"

func StringToUUID(s string) *uuid.UUID {
	uuid := uuid.MustParse(s)
	return &uuid
}

func Pointer[T any](v T) *T {
	return &v
}

func BoolPtr(b bool) *bool {
	return &b
}

func StringToPtr(s string) *string {
	return &s
}

func UintPtr(v uint) *uint {
	return &v
}

func Uint32ToPtr(v uint32) *uint32 {
	return &v
}

func UintTo64Ptr(v uint64) *uint64 {
	return &v
}

func PtrToUint64(v *uint64) uint64 {
	if v == nil {
		return 0
	}
	return *v
}
