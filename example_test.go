package gommatranslate_test

import (
	"fmt"

	"github.com/eve-learn/gommatranslate"
)

func ExampleLanguages() {
	codes := gommatranslate.Languages()
	fmt.Println(len(codes) > 0)
	// Output:
	// true
}
