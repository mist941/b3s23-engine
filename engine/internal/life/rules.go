package life

func Next(alive bool, neighbours int) bool {
	return neighbours == 3 || (alive && neighbours == 2)
}
