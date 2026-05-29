package computer

import "strings"

func normalizeScroll(direction string, amount int) (string, int, bool, error) {
	if amount == 0 {
		return "", 0, false, RuntimeError{Code: "invalid_args", Message: "--amount must not be zero"}
	}
	direction = strings.ToLower(strings.TrimSpace(direction))
	if direction == "" {
		if amount > 0 {
			return "up", amount, false, nil
		}
		return "down", amount, false, nil
	}
	if amount < 0 {
		amount = -amount
	}
	switch direction {
	case "up":
		return direction, amount, false, nil
	case "down":
		return direction, -amount, false, nil
	case "right":
		return direction, amount, true, nil
	case "left":
		return direction, -amount, true, nil
	default:
		return "", 0, false, RuntimeError{Code: "invalid_args", Message: "--direction must be up, down, left, or right"}
	}
}
