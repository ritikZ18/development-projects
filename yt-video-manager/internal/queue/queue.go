// this will be in-memory list of links and their state stored 
// FUTURE TODO : in-mem list of downloads and can be played from the terminal with no-ads


package queue

// status will be like lifecycle state of Q'ed items
type Status string

const ( 
	Pending Status = "pending"
	Checking Status = "checking"
	Checked Status = "checked"
	Downloading Status = "downloading"
	Done Status = "done"
	Failed Status = "failed"
)

// if item is single link in Q
type Item struct { 
	ID int 
	URL string 
	Title string
	Status Status
	Progress float64 // 0-100
	Err string
}

type Queue struct { 
	items []*Item
	nextID int 
}

func New() *Queue{ return &Queue{nextID:1}}

func(q *Queue) Add(url string ) *Item { 
	it := &Item{ID: q.nextID, URL: url, Status: Pending}
	q.nextID++
	q.items = append(q.items, it)
	return it
}

func ( q *Queue) Items() []*Item { return q.items }

func ( q *Queue) Get(id int) *Item { 
	for _, it := range q.items { 
		if it.ID == id { 
			return it 
		}
	}
	return nil
}

func  ( q *Queue) Remove(id int) bool { 
	for i, it := range q.items { 
		if it.ID == id { 
			q.items = append(q.items[:i], q.items[i+1:]...)
			return true
		}
	}
	return false 
}

func (q *Queue) Clear() { q.items = nil }

func(q *Queue) Len() int { return len(q.items)}