package logger

type Printable interface {
	string | []byte
}

func Trunc[P Printable](p P, lim int) P {
	if len(p) <= lim {
		return p
	}
	return p[:lim] // + P("...")
}
