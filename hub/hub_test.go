package hub

func createDummy() Hub {
	return NewHub([]byte("publisher"), []byte("publisher"))
}
