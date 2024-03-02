package luhn

func IsValid(number int) bool {
	var sum int
	for i := 0; number > 0; i++ {
		last := number % 10
		if i%2 != 0 {
			last *= 2
			if last > 9 {
				last -= 9
			}
		}
		sum += last
		number /= 10
	}
	return sum%10 == 0
}
