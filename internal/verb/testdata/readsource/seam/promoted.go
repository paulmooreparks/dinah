package bench

// plantReader is Source under an unexported name.
type plantReader = Source

// PlantBox holds a bench's source in an embedded, unexported field.
type PlantBox struct{ plantReader }

// PlantBox hands the bench's source out inside the box.
func (b *Bench) PlantBox() PlantBox { return PlantBox{b.source()} }
