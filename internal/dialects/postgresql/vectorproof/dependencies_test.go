package vectorproof

import "testing"

func TestDependencyCyclesAndBudgets(t *testing.T) {
	d := &dependencyCheck{seen: map[Reference]bool{}}
	ref := Reference{"pg_type", 1}
	for i := 0; i < MaxDependencyEdges; i++ {
		if err := d.add(ref); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.queue) != 1 || d.add(ref) == nil {
		t.Fatal("cycles must deduplicate objects but still consume edge budget")
	}
	d = &dependencyCheck{seen: map[Reference]bool{}}
	for i := 1; i <= MaxDependencies; i++ {
		if err := d.add(Reference{"pg_type", uint32(i)}); err != nil {
			t.Fatal(err)
		}
	}
	if d.add(Reference{"pg_proc", 1}) == nil {
		t.Fatal("class-qualified dependency exceeded the object budget")
	}
}
