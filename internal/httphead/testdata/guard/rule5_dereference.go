package guard

type indirect struct {
	Affordances *[]string
}

func rewrite(held *indirect) {
	(*held.Affordances)[0] = "next"
}
