package cache

import "testing"

func TestShardAddAndGet(t *testing.T) {
	s := newShard(4)
	s.Add("1", 1)

	if _, found := s.Get("1"); !found {
		t.Fatal("Failed to find inserted record")
	}
}

func TestShardLen(t *testing.T) {
	s := newShard(4)

	s.Add("1", 1)
	if l := s.Len(); l != 1 {
		t.Fatalf("Shard size should %d, got %d", 1, l)
	}

	s.Add("1", 1)
	if l := s.Len(); l != 1 {
		t.Fatalf("Shard size should %d, got %d", 1, l)
	}

	s.Add("2", 2)
	if l := s.Len(); l != 2 {
		t.Fatalf("Shard size should %d, got %d", 2, l)
	}
}

func TestShardEvict(t *testing.T) {
	s := newShard(1)
	s.Add("1", 1)
	s.Add("2", 2)
	// 1 should be gone

	if _, found := s.Get("1"); found {
		t.Fatal("Found item that should have been evicted")
	}
}

func TestShardLenEvict(t *testing.T) {
	s := newShard(4)
	s.Add("1", 1)
	s.Add("2", 1)
	s.Add("3", 1)
	s.Add("4", 1)

	if l := s.Len(); l != 4 {
		t.Fatalf("Shard size should %d, got %d", 4, l)
	}

	// This should evict one element
	s.Add("5", 1)
	if l := s.Len(); l != 4 {
		t.Fatalf("Shard size should %d, got %d", 4, l)
	}
}

func TestShardUpdateAdd(t *testing.T) {
	s := newShard(1)

	updateFunc := func(el interface{}) interface{} {
		iptr := el.(*int)
		*iptr += 1
		el = iptr
		return iptr
	}

	addFunc := func() interface{} {
		i := 1
		return &i
	}

	// first call should insert value returned by add
	el := s.UpdateAdd("a", updateFunc, addFunc)

	el, found := s.Get("a")
	if !found {
		t.Fatal("failed to find inserted record")
	}
	i := el.(*int)
	if *i != 1 {
		t.Fatalf("expected to see inital value of 1, got %v", *i)
	}

	// second call should increment the value, and return it
	el = s.UpdateAdd("a", updateFunc, addFunc)

	i = el.(*int)

	if *i != 2 {
		t.Fatalf("expected to see return value of 2, got %v", i)
	}
	el, found = s.Get("a")
	if !found {
		t.Fatal("failed to find inserted record")
	}
	i = el.(*int)
	if *i != 2 {
		t.Fatalf("expected to see value incremented to 2, got %v", i)
	}

	// Adding another key should evict the prior key
	el = s.UpdateAdd("b", updateFunc, addFunc)
	el, found = s.Get("a")
	if found {
		t.Fatal("expected 'a' to be evicted, but found it")
	}
	el, found = s.Get("b")
	if !found {
		t.Fatal("failed to find inserted record")
	}

}
