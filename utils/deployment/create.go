package deployment

func lables(name string, cr string) map[string]string {
	return map[string]string{"name": name, "cr": cr}
}
