package csstree

import (
	"errors"
	"iter"
)

// ErrForeignItem is the panic value used when an item that does not belong to
// a list is used as an anchor for insertion or removal, mirroring css-tree's
// "item doesn't belong to list" errors.
var ErrForeignItem = errors.New("csstree: item doesn't belong to list")

// ListItem is an element of a List.
type ListItem struct {
	Prev *ListItem
	Next *ListItem
	Data Node
}

// listCursor is an iteration position. Cursors of a list form a stack so that
// nested iterations survive mutation of the list (css-tree's cursor model).
type listCursor struct {
	prev   *ListItem
	next   *ListItem
	parent *listCursor
}

// List is css-tree's doubly linked list of nodes. Iterating it with ForEach,
// ForEachRight, Reduce, ReduceRight, NextUntil or PrevUntil is safe while the
// callback inserts or removes items: active cursors are updated on every
// mutation, exactly as in css-tree.
type List struct {
	Head   *ListItem
	Tail   *ListItem
	cursor *listCursor
}

// NewList returns an empty list.
func NewList() *List {
	return &List{}
}

// NewListOf returns a list holding nodes in order.
func NewListOf(nodes ...Node) *List {
	return NewList().FromSlice(nodes)
}

// NewListItem wraps data in a detached list item.
func NewListItem(data Node) *ListItem {
	return &ListItem{Data: data}
}

func (l *List) allocateCursor(prev, next *ListItem) *listCursor {
	c := &listCursor{prev: prev, next: next, parent: l.cursor}
	l.cursor = c
	return c
}

func (l *List) releaseCursor() {
	l.cursor = l.cursor.parent
}

func (l *List) updateCursors(prevOld, prevNew, nextOld, nextNew *ListItem) {
	for c := l.cursor; c != nil; c = c.parent {
		if c.prev == prevOld {
			c.prev = prevNew
		}
		if c.next == nextOld {
			c.next = nextNew
		}
	}
}

// updatePrevCursors is updateCursors for the case where css-tree passes no
// next arguments (an undefined nextOld never matches a cursor).
func (l *List) updatePrevCursors(prevOld, prevNew *ListItem) {
	for c := l.cursor; c != nil; c = c.parent {
		if c.prev == prevOld {
			c.prev = prevNew
		}
	}
}

// All iterates over the list data from head to tail. Like css-tree's
// Symbol.iterator it follows Next links directly and is not cursor-safe.
func (l *List) All() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for it := l.Head; it != nil; it = it.Next {
			if !yield(it.Data) {
				return
			}
		}
	}
}

// Size returns the number of items.
func (l *List) Size() int {
	n := 0
	for it := l.Head; it != nil; it = it.Next {
		n++
	}
	return n
}

// IsEmpty reports whether the list has no items.
func (l *List) IsEmpty() bool {
	return l.Head == nil
}

// First returns the data of the first item, or nil.
func (l *List) First() Node {
	if l.Head == nil {
		return nil
	}
	return l.Head.Data
}

// Last returns the data of the last item, or nil.
func (l *List) Last() Node {
	if l.Tail == nil {
		return nil
	}
	return l.Tail.Data
}

// FromSlice replaces the list content with nodes and returns the list.
func (l *List) FromSlice(nodes []Node) *List {
	var cursor *ListItem
	l.Head = nil
	for _, data := range nodes {
		item := NewListItem(data)
		if cursor != nil {
			cursor.Next = item
		} else {
			l.Head = item
		}
		item.Prev = cursor
		cursor = item
	}
	l.Tail = cursor
	return l
}

// ToSlice returns the list data as a slice.
func (l *List) ToSlice() []Node {
	var out []Node
	for it := l.Head; it != nil; it = it.Next {
		out = append(out, it.Data)
	}
	return out
}

// ForEach calls fn for every item from head to tail.
func (l *List) ForEach(fn func(data Node, item *ListItem)) {
	c := l.allocateCursor(nil, l.Head)
	for c.next != nil {
		item := c.next
		c.next = item.Next
		fn(item.Data, item)
	}
	l.releaseCursor()
}

// ForEachRight calls fn for every item from tail to head.
func (l *List) ForEachRight(fn func(data Node, item *ListItem)) {
	c := l.allocateCursor(l.Tail, nil)
	for c.prev != nil {
		item := c.prev
		c.prev = item.Prev
		fn(item.Data, item)
	}
	l.releaseCursor()
}

// Reduce folds the list from head to tail.
func Reduce[T any](l *List, fn func(acc T, data Node, item *ListItem) T, initial T) T {
	c := l.allocateCursor(nil, l.Head)
	acc := initial
	for c.next != nil {
		item := c.next
		c.next = item.Next
		acc = fn(acc, item.Data, item)
	}
	l.releaseCursor()
	return acc
}

// ReduceRight folds the list from tail to head.
func ReduceRight[T any](l *List, fn func(acc T, data Node, item *ListItem) T, initial T) T {
	c := l.allocateCursor(l.Tail, nil)
	acc := initial
	for c.prev != nil {
		item := c.prev
		c.prev = item.Prev
		acc = fn(acc, item.Data, item)
	}
	l.releaseCursor()
	return acc
}

// Some reports whether fn returns true for any item.
func (l *List) Some(fn func(data Node, item *ListItem) bool) bool {
	for it := l.Head; it != nil; it = it.Next {
		if fn(it.Data, it) {
			return true
		}
	}
	return false
}

// Map returns a new list with the results of fn.
func (l *List) Map(fn func(data Node, item *ListItem) Node) *List {
	result := NewList()
	for it := l.Head; it != nil; it = it.Next {
		result.AppendData(fn(it.Data, it))
	}
	return result
}

// Filter returns a new list with the data for which fn returns true.
func (l *List) Filter(fn func(data Node, item *ListItem) bool) *List {
	result := NewList()
	for it := l.Head; it != nil; it = it.Next {
		if fn(it.Data, it) {
			result.AppendData(it.Data)
		}
	}
	return result
}

// NextUntil calls fn for start and the items after it until fn returns true.
func (l *List) NextUntil(start *ListItem, fn func(data Node, item *ListItem) bool) {
	if start == nil {
		return
	}
	c := l.allocateCursor(nil, start)
	for c.next != nil {
		item := c.next
		c.next = item.Next
		if fn(item.Data, item) {
			break
		}
	}
	l.releaseCursor()
}

// PrevUntil calls fn for start and the items before it until fn returns true.
func (l *List) PrevUntil(start *ListItem, fn func(data Node, item *ListItem) bool) {
	if start == nil {
		return
	}
	c := l.allocateCursor(start, nil)
	for c.prev != nil {
		item := c.prev
		c.prev = item.Prev
		if fn(item.Data, item) {
			break
		}
	}
	l.releaseCursor()
}

// Clear drops every item.
func (l *List) Clear() {
	l.Head = nil
	l.Tail = nil
}

// Copy returns a shallow copy of the list (same data, new items).
func (l *List) Copy() *List {
	result := NewList()
	for it := l.Head; it != nil; it = it.Next {
		result.AppendData(it.Data)
	}
	return result
}

// Prepend inserts item at the beginning of the list.
func (l *List) Prepend(item *ListItem) *List {
	l.updateCursors(nil, item, l.Head, item)
	if l.Head != nil {
		l.Head.Prev = item
		item.Next = l.Head
	} else {
		l.Tail = item
	}
	l.Head = item
	return l
}

// PrependData inserts data at the beginning of the list.
func (l *List) PrependData(data Node) *List {
	return l.Prepend(NewListItem(data))
}

// Append inserts item at the end of the list.
func (l *List) Append(item *ListItem) *List {
	return l.Insert(item, nil)
}

// AppendData inserts data at the end of the list.
func (l *List) AppendData(data Node) *List {
	return l.Insert(NewListItem(data), nil)
}

// Insert inserts item before the before item, or at the end when before is
// nil. It panics with ErrForeignItem when before belongs to another list.
func (l *List) Insert(item, before *ListItem) *List {
	if before == nil {
		l.updateCursors(l.Tail, item, nil, item)
		if l.Tail != nil {
			l.Tail.Next = item
			item.Prev = l.Tail
		} else {
			l.Head = item
		}
		l.Tail = item
		return l
	}

	l.updateCursors(before.Prev, item, before, item)
	if before.Prev == nil {
		if l.Head != before {
			panic(ErrForeignItem)
		}
		l.Head = item
		before.Prev = item
		item.Next = before
		l.updatePrevCursors(nil, item)
	} else {
		before.Prev.Next = item
		item.Prev = before.Prev
		before.Prev = item
		item.Next = before
	}
	return l
}

// InsertData inserts data before the before item (at the end when nil).
func (l *List) InsertData(data Node, before *ListItem) *List {
	return l.Insert(NewListItem(data), before)
}

// Remove unlinks item from the list and returns it. It panics with
// ErrForeignItem when item belongs to another list.
func (l *List) Remove(item *ListItem) *ListItem {
	l.updateCursors(item, item.Prev, item, item.Next)

	if item.Prev != nil {
		item.Prev.Next = item.Next
	} else {
		if l.Head != item {
			panic(ErrForeignItem)
		}
		l.Head = item.Next
	}

	if item.Next != nil {
		item.Next.Prev = item.Prev
	} else {
		if l.Tail != item {
			panic(ErrForeignItem)
		}
		l.Tail = item.Prev
	}

	item.Prev = nil
	item.Next = nil
	return item
}

// Push appends data to the list.
func (l *List) Push(data Node) {
	l.Insert(NewListItem(data), nil)
}

// Pop removes and returns the last item, or nil for an empty list.
func (l *List) Pop() *ListItem {
	if l.Tail == nil {
		return nil
	}
	return l.Remove(l.Tail)
}

// Unshift prepends data to the list.
func (l *List) Unshift(data Node) {
	l.Prepend(NewListItem(data))
}

// Shift removes and returns the first item, or nil for an empty list.
func (l *List) Shift() *ListItem {
	if l.Head == nil {
		return nil
	}
	return l.Remove(l.Head)
}

// PrependList moves all items of list to the beginning of l.
func (l *List) PrependList(list *List) *List {
	return l.InsertList(list, l.Head)
}

// AppendList moves all items of list to the end of l.
func (l *List) AppendList(list *List) *List {
	return l.InsertList(list, nil)
}

// InsertList moves all items of list before the before item (or to the end
// when before is nil); list is left empty.
func (l *List) InsertList(list *List, before *ListItem) *List {
	if list.Head == nil {
		return l
	}

	if before != nil {
		l.updateCursors(before.Prev, list.Tail, before, list.Head)
		if before.Prev != nil {
			before.Prev.Next = list.Head
			list.Head.Prev = before.Prev
		} else {
			l.Head = list.Head
		}
		before.Prev = list.Tail
		list.Tail.Next = before
	} else {
		l.updateCursors(l.Tail, list.Tail, nil, list.Head)
		if l.Tail != nil {
			l.Tail.Next = list.Head
			list.Head.Prev = l.Tail
		} else {
			l.Head = list.Head
		}
		l.Tail = list.Tail
	}

	list.Head = nil
	list.Tail = nil
	return l
}

// Replace puts newItem in place of oldItem.
func (l *List) Replace(oldItem, newItem *ListItem) {
	l.Insert(newItem, oldItem)
	l.Remove(oldItem)
}

// ReplaceWithList puts all items of list in place of oldItem.
func (l *List) ReplaceWithList(oldItem *ListItem, list *List) {
	l.InsertList(list, oldItem)
	l.Remove(oldItem)
}
