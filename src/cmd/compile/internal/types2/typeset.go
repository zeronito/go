// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"bytes"
	"cmd/compile/internal/syntax"
	"fmt"
	"sort"
)

// ----------------------------------------------------------------------------
// API

// A TypeSet represents the type set of an interface.
type TypeSet struct {
	comparable bool // if set, the interface is or embeds comparable
	// TODO(gri) consider using a set for the methods for faster lookup
	methods []*Func  // all methods of the interface; sorted by unique ID
	terms   termlist // type terms of the type set
}

// IsEmpty reports whether type set s is the empty set.
func (s *TypeSet) IsEmpty() bool { return s.terms.isEmpty() }

// IsTop reports whether type set s is the set of all types (corresponding to the empty interface).
func (s *TypeSet) IsTop() bool { return !s.comparable && len(s.methods) == 0 && s.terms.isTop() }

// TODO(gri) IsMethodSet is not a great name for this predicate. Find a better one.

// IsMethodSet reports whether the type set s is described by a single set of methods.
func (s *TypeSet) IsMethodSet() bool { return !s.comparable && s.terms.isTop() }

// IsComparable reports whether each type in the set is comparable.
func (s *TypeSet) IsComparable() bool {
	if s.terms.isTop() {
		return s.comparable
	}
	return s.is(func(t *term) bool {
		return Comparable(t.typ)
	})
}

// TODO(gri) IsTypeSet is not a great name for this predicate. Find a better one.

// IsTypeSet reports whether the type set s is represented by a finite set of underlying types.
func (s *TypeSet) IsTypeSet() bool {
	return !s.comparable && len(s.methods) == 0
}

// NumMethods returns the number of methods available.
func (s *TypeSet) NumMethods() int { return len(s.methods) }

// Method returns the i'th method of type set s for 0 <= i < s.NumMethods().
// The methods are ordered by their unique ID.
func (s *TypeSet) Method(i int) *Func { return s.methods[i] }

// LookupMethod returns the index of and method with matching package and name, or (-1, nil).
func (s *TypeSet) LookupMethod(pkg *Package, name string) (int, *Func) {
	// TODO(gri) s.methods is sorted - consider binary search
	return lookupMethod(s.methods, pkg, name)
}

func (s *TypeSet) String() string {
	switch {
	case s.IsEmpty():
		return "∅"
	case s.IsTop():
		return "⊤"
	}

	hasMethods := len(s.methods) > 0
	hasTerms := s.hasTerms()

	var buf bytes.Buffer
	buf.WriteByte('{')
	if s.comparable {
		buf.WriteString(" comparable")
		if hasMethods || hasTerms {
			buf.WriteByte(';')
		}
	}
	for i, m := range s.methods {
		if i > 0 {
			buf.WriteByte(';')
		}
		buf.WriteByte(' ')
		buf.WriteString(m.String())
	}
	if hasMethods && hasTerms {
		buf.WriteByte(';')
	}
	if hasTerms {
		buf.WriteString(s.terms.String())
	}
	buf.WriteString(" }") // there was at least one method or term

	return buf.String()
}

// ----------------------------------------------------------------------------
// Implementation

func (s *TypeSet) hasTerms() bool             { return !s.terms.isTop() }
func (s *TypeSet) structuralType() Type       { return s.terms.structuralType() }
func (s *TypeSet) includes(t Type) bool       { return s.terms.includes(t) }
func (s1 *TypeSet) subsetOf(s2 *TypeSet) bool { return s1.terms.subsetOf(s2.terms) }

// TODO(gri) TypeSet.is and TypeSet.underIs should probably also go into termlist.go

var topTerm = term{false, theTop}

func (s *TypeSet) is(f func(*term) bool) bool {
	if len(s.terms) == 0 {
		return false
	}
	for _, t := range s.terms {
		// Terms represent the top term with a nil type.
		// The rest of the type checker uses the top type
		// instead. Convert.
		// TODO(gri) investigate if we can do without this
		if t.typ == nil {
			t = &topTerm
		}
		if !f(t) {
			return false
		}
	}
	return true
}

func (s *TypeSet) underIs(f func(Type) bool) bool {
	if len(s.terms) == 0 {
		return false
	}
	for _, t := range s.terms {
		// see corresponding comment in TypeSet.is
		u := t.typ
		if u == nil {
			u = theTop
		}
		// t == under(t) for ~t terms
		if !t.tilde {
			u = under(u)
		}
		if debug {
			assert(Identical(u, under(u)))
		}
		if !f(u) {
			return false
		}
	}
	return true
}

// topTypeSet may be used as type set for the empty interface.
var topTypeSet = TypeSet{terms: topTermlist}

// computeInterfaceTypeSet may be called with check == nil.
func computeInterfaceTypeSet(check *Checker, pos syntax.Pos, ityp *Interface) *TypeSet {
	if ityp.tset != nil {
		return ityp.tset
	}

	// If the interface is not fully set up yet, the type set will
	// not be complete, which may lead to errors when using the the
	// type set (e.g. missing method). Don't compute a partial type
	// set (and don't store it!), so that we still compute the full
	// type set eventually. Instead, return the top type set and
	// let any follow-on errors play out.
	if !ityp.complete {
		return &topTypeSet
	}

	if check != nil && check.conf.Trace {
		// Types don't generally have position information.
		// If we don't have a valid pos provided, try to use
		// one close enough.
		if !pos.IsKnown() && len(ityp.methods) > 0 {
			pos = ityp.methods[0].pos
		}

		check.trace(pos, "type set for %s", ityp)
		check.indent++
		defer func() {
			check.indent--
			check.trace(pos, "=> %s ", ityp.typeSet())
		}()
	}

	// An infinitely expanding interface (due to a cycle) is detected
	// elsewhere (Checker.validType), so here we simply assume we only
	// have valid interfaces. Mark the interface as complete to avoid
	// infinite recursion if the validType check occurs later for some
	// reason.
	ityp.tset = &TypeSet{terms: topTermlist} // TODO(gri) is this sufficient?

	// Methods of embedded interfaces are collected unchanged; i.e., the identity
	// of a method I.m's Func Object of an interface I is the same as that of
	// the method m in an interface that embeds interface I. On the other hand,
	// if a method is embedded via multiple overlapping embedded interfaces, we
	// don't provide a guarantee which "original m" got chosen for the embedding
	// interface. See also issue #34421.
	//
	// If we don't care to provide this identity guarantee anymore, instead of
	// reusing the original method in embeddings, we can clone the method's Func
	// Object and give it the position of a corresponding embedded interface. Then
	// we can get rid of the mpos map below and simply use the cloned method's
	// position.

	var todo []*Func
	var seen objset
	var methods []*Func
	mpos := make(map[*Func]syntax.Pos) // method specification or method embedding position, for good error messages
	addMethod := func(pos syntax.Pos, m *Func, explicit bool) {
		switch other := seen.insert(m); {
		case other == nil:
			methods = append(methods, m)
			mpos[m] = pos
		case explicit:
			if check == nil {
				panic(fmt.Sprintf("%s: duplicate method %s", m.pos, m.name))
			}
			// check != nil
			var err error_
			err.errorf(pos, "duplicate method %s", m.name)
			err.errorf(mpos[other.(*Func)], "other declaration of %s", m.name)
			check.report(&err)
		default:
			// We have a duplicate method name in an embedded (not explicitly declared) method.
			// Check method signatures after all types are computed (issue #33656).
			// If we're pre-go1.14 (overlapping embeddings are not permitted), report that
			// error here as well (even though we could do it eagerly) because it's the same
			// error message.
			if check == nil {
				// check method signatures after all locally embedded interfaces are computed
				todo = append(todo, m, other.(*Func))
				break
			}
			// check != nil
			check.later(func() {
				if !check.allowVersion(m.pkg, 1, 14) || !Identical(m.typ, other.Type()) {
					var err error_
					err.errorf(pos, "duplicate method %s", m.name)
					err.errorf(mpos[other.(*Func)], "other declaration of %s", m.name)
					check.report(&err)
				}
			})
		}
	}

	for _, m := range ityp.methods {
		addMethod(m.pos, m, true)
	}

	// collect embedded elements
	var allTerms = topTermlist
	for i, typ := range ityp.embeddeds {
		// The embedding position is nil for imported interfaces
		// and also for interface copies after substitution (but
		// in that case we don't need to report errors again).
		var pos syntax.Pos // embedding position
		if ityp.embedPos != nil {
			pos = (*ityp.embedPos)[i]
		}
		var terms termlist
		switch t := under(typ).(type) {
		case *Interface:
			tset := computeInterfaceTypeSet(check, pos, t)
			if tset.comparable {
				ityp.tset.comparable = true
			}
			for _, m := range tset.methods {
				addMethod(pos, m, false) // use embedding position pos rather than m.pos
			}
			terms = tset.terms
		case *Union:
			tset := computeUnionTypeSet(check, pos, t)
			terms = tset.terms
		case *TypeParam:
			// Embedding stand-alone type parameters is not permitted.
			// This case is handled during union parsing.
			unreachable()
		default:
			if typ == Typ[Invalid] {
				continue
			}
			if check != nil && !check.allowVersion(check.pkg, 1, 18) {
				check.errorf(pos, "%s is not an interface", typ)
				continue
			}
			terms = termlist{{false, typ}}
		}
		// The type set of an interface is the intersection
		// of the type sets of all its elements.
		allTerms = allTerms.intersect(terms)
	}
	ityp.embedPos = nil // not needed anymore (errors have been reported)

	// process todo's (this only happens if check == nil)
	for i := 0; i < len(todo); i += 2 {
		m := todo[i]
		other := todo[i+1]
		if !Identical(m.typ, other.typ) {
			panic(fmt.Sprintf("%s: duplicate method %s", m.pos, m.name))
		}
	}

	if methods != nil {
		sortMethods(methods)
		ityp.tset.methods = methods
	}
	ityp.tset.terms = allTerms

	return ityp.tset
}

func sortMethods(list []*Func) {
	sort.Sort(byUniqueMethodName(list))
}

func assertSortedMethods(list []*Func) {
	if !debug {
		panic("internal error: assertSortedMethods called outside debug mode")
	}
	if !sort.IsSorted(byUniqueMethodName(list)) {
		panic("internal error: methods not sorted")
	}
}

// byUniqueMethodName method lists can be sorted by their unique method names.
type byUniqueMethodName []*Func

func (a byUniqueMethodName) Len() int           { return len(a) }
func (a byUniqueMethodName) Less(i, j int) bool { return a[i].less(&a[j].object) }
func (a byUniqueMethodName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// computeUnionTypeSet may be called with check == nil.
func computeUnionTypeSet(check *Checker, pos syntax.Pos, utyp *Union) *TypeSet {
	if utyp.tset != nil {
		return utyp.tset
	}

	// avoid infinite recursion (see also computeInterfaceTypeSet)
	utyp.tset = new(TypeSet)

	var allTerms termlist
	for _, t := range utyp.terms {
		var terms termlist
		switch u := under(t.typ).(type) {
		case *Interface:
			terms = computeInterfaceTypeSet(check, pos, u).terms
		case *TypeParam:
			// A stand-alone type parameters is not permitted as union term.
			// This case is handled during union parsing.
			unreachable()
		default:
			terms = termlist{t}
		}
		// The type set of a union expression is the union
		// of the type sets of each term.
		allTerms = allTerms.union(terms)
	}
	utyp.tset.terms = allTerms

	return utyp.tset
}
