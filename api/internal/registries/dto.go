package registries

type RegistryNumberDTO struct {
	CourseID   int64 `json:"courseId"`
	Year       int64 `json:"year"`
	NextNumber int64 `json:"nextNumber"`
}

type ResponseNumber struct {
	Data RegistryNumberDTO `json:"data"`
}
