package currency

func ConvertToPrimary(v int) float64 {
	return float64(v) / 1000
}

func ConvertToSubunit(v float64) int {
	return int(v * 1000)
}
