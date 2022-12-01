package messenger

func RemovePartialOccurrence[T interface{}](a []T, f func(T) bool) []T {
	for i, v := range a {
		if f(v) {
			a = append(a[:i], a[i+1:]...)
			continue
		}
	}
	return a
}
