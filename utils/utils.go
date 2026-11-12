package utils

import (
	"reflect"
)

// SanitizeDefaults walks through any struct and initializes nil slices and maps
// to empty ones ([] and {} in JSON instead of null). Nil pointers are left alone.
func SanitizeDefaults(v interface{}) {
	if v == nil {
		return
	}

	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr {
		return
	}

	sanitizeReflect(val.Elem())
}

func sanitizeReflect(val reflect.Value) {
	if !val.IsValid() || !val.CanSet() {
		return
	}

	switch val.Kind() {
	case reflect.Slice:
		if val.IsNil() {
			// Turns nil slice into []
			val.Set(reflect.MakeSlice(val.Type(), 0, 0))
			return
		}
		for i := 0; i < val.Len(); i++ {
			sanitizeReflect(val.Index(i))
		}

	case reflect.Map:
		if val.IsNil() {
			// Turns nil map into {}
			val.Set(reflect.MakeMap(val.Type()))
		}

	case reflect.Ptr:
		// A nil pointer means "no value" (null in JSON) and stays that way.
		if !val.IsNil() {
			sanitizeReflect(val.Elem())
		}

	case reflect.Struct:
		for i := 0; i < val.NumField(); i++ {
			sanitizeReflect(val.Field(i))
		}
	}
}
