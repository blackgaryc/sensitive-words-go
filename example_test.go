package sensitivewords

import "fmt"

func ExampleMatcher() {
	entries := []Entry{
		{Word: "习近平", Category: "政治", Source: "政治类型.txt"},
		{Word: "法轮功", Category: "暴恐", Source: "暴恐词库.txt"},
	}

	matcher, err := NewMatcher(entries, Options{})
	if err != nil {
		panic(err)
	}

	result := matcher.Detect("这段文本里有习-近-平")
	fmt.Println(result.Violates)
	fmt.Println(result.Matches[0].Matched)
	fmt.Println(result.Matches[0].Mode)

	// Output:
	// true
	// 习-近-平
	// compact
}

func ExampleNewEmptyMatcher() {
	matcher := NewEmptyMatcher(Options{})
	matcher.AddWords([]string{"习近平", "法轮功"})

	fmt.Println(matcher.Count())
	fmt.Println(matcher.Contains("这里有法.轮.功"))

	// Output:
	// 2
	// true
}
