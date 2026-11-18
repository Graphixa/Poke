// Adapted from https://github.com/erik-adelbert/firework (MIT)

package helper

import "iter"

func EmptySeq[T any]() iter.Seq[T] {
	return func(yield func(T) bool) {}
}
