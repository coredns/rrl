package cache

import "testing"

func TestCacheAddGetRemove(t *testing.T) {
	c := New(4)
	c.Add("1", 1)

	if _, found := c.Get("1"); !found {
		t.Fatal("failed to find inserted record")
	}

	c.Remove("1")

	if _, found := c.Get("1"); found {
		t.Fatal("failed to remove inserted record")
	}
}

func TestCacheUpdateAdd(t *testing.T) {
	c := New(4)

	updateFunc := func(el *interface{}) interface{} {
		i := (*el).(int)
		i += 1
		*el = i
		return i
	}

	addFunc := func() interface{} {
		return 1
	}

	// first call should insert value returned by add
	el := c.UpdateAdd("a", updateFunc, addFunc)

	el, found := c.Get("a")
	if !found {
		t.Fatal("failed to find inserted record")
	}
	i := (el).(int)
	if i != 1 {
		t.Fatalf("expected to see inital value of 1, got %v", i)
	}

	// second call should increment the value, and return it
	el = c.UpdateAdd("a", updateFunc, addFunc)

	i = (el).(int)

	if i != 2 {
		t.Fatalf("expected to see return value of 2, got %v", i)
	}
	el, found = c.Get("a")
	if !found {
		t.Fatal("failed to find inserted record")
	}
	i = (el).(int)
	if i != 2 {
		t.Fatalf("expected to see value incremented to 2, got %v", i)
	}
}

func TestCacheLen(t *testing.T) {
	c := New(4)

	c.Add("1", 1)
	if l := c.Len(); l != 1 {
		t.Fatalf("cache size should %d, got %d", 1, l)
	}

	c.Add("1", 1)
	if l := c.Len(); l != 1 {
		t.Fatalf("cache size should %d, got %d", 1, l)
	}

	c.Add("2", 2)
	if l := c.Len(); l != 2 {
		t.Fatalf("cache size should %d, got %d", 2, l)
	}
}

func BenchmarkCache(b *testing.B) {
	b.ReportAllocs()

	c := New(4)
	for n := 0; n < b.N; n++ {
		c.Add("1", 1)
		c.Get("1")
	}
}
