package guard

type counted struct {
	Affordances []int
}

func bump(c *counted) {
	c.Affordances[0]++
}
