package exec

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"qlang.io/qlang.spec.v1"
)

var (
	// ErrNewWithoutClassName is returned when new doesn't specify a class.
	ErrNewWithoutClassName = errors.New("new object without class name")

	// ErrNewObjectWithNotClass is returned when new T but T is not a class.
	ErrNewObjectWithNotClass = errors.New("can't new object: not a class")

	// ErrRefWithoutObject is returned when refer member without specified an object.
	ErrRefWithoutObject = errors.New("reference without object")
)

// -----------------------------------------------------------------------------

// A Class represents a qlang class.
//
type Class struct {
	Fns map[string]*Function
}

// Exec is required by interface Instr.
//
func (p *Class) Exec(stk *Stack, ctx *Context) {

	for _, f := range p.Fns {
		f.parent = ctx
	}
	stk.Push(p)
}

// IClass returns a Class instruction.
//
func IClass() *Class {

	fns := make(map[string]*Function)
	return &Class{Fns: fns}
}

// -----------------------------------------------------------------------------

// A Object represents a qlang object.
//
type Object struct {
	vars map[string]interface{}
	Cls  *Class
}

// SetVar sets the value of a qlang object's member.
//
func (p *Object) SetVar(name string, val interface{}) {

	if _, ok := p.Cls.Fns[name]; ok {
		panic("set failed: class already have a method named " + name)
	}
	p.vars[name] = val
}

// SetMemberVar implements set(object, k1, v1, k2, v2, ...), ie. sets values of qlang object's multiple members.
//
func SetMemberVar(m interface{}, args ...interface{}) {

	if v, ok := m.(*Object); ok {
		for i := 0; i < len(args); i += 2 {
			v.SetVar(args[i].(string), args[i+1])
		}
		return
	}
	panic(fmt.Sprintf("type `%v` doesn't support `set` operator", reflect.TypeOf(m)))
}

func init() {
	qlang.SetEx = SetMemberVar
}

// -----------------------------------------------------------------------------

type thisDeref struct {
	this *Object
	fn   *Function
}

func (p *thisDeref) Call(a ...interface{}) interface{} {

	args := make([]interface{}, len(a)+1)
	args[0] = p.this
	for i, v := range a {
		args[i+1] = v
	}
	return p.fn.Call(args...)
}

// -----------------------------------------------------------------------------

type iNew int

func (nArgs iNew) Exec(stk *Stack, ctx *Context) {

	var args []interface{}

	if nArgs != 0 {
		args = stk.PopNArgs(int(nArgs))
	}

	if v, ok := stk.Pop(); ok {
		if cls, ok := v.(*Class); ok {
			obj := &Object{
				vars: make(map[string]interface{}),
				Cls:  cls,
			}
			if init, ok := cls.Fns["_init"]; ok { // 构造函数
				closure := &thisDeref{
					this: obj,
					fn:   init,
				}
				closure.Call(args...)
			}
			stk.Push(obj)
			return
		}
		panic(ErrNewObjectWithNotClass)
	}
	panic(ErrNewWithoutClassName)
}

// INew returns a New instruction.
//
func INew(nArgs int) Instr {
	return iNew(nArgs)
}

// -----------------------------------------------------------------------------

type iMemberRef struct {
	name string
}

var (
	typeObjectPtr = reflect.TypeOf((*Object)(nil))
	typeClassPtr  = reflect.TypeOf((*Class)(nil))
)

func (p *iMemberRef) Exec(stk *Stack, ctx *Context) {

	v, ok := stk.Pop()
	if !ok {
		panic(ErrRefWithoutObject)
	}

	name := p.name
	t := reflect.TypeOf(v)
	switch t {
	case typeObjectPtr:
		o := v.(*Object)
		val, ok := o.vars[name]
		if !ok {
			if fn, ok := o.Cls.Fns[name]; ok {
				t := &thisDeref{
					this: o,
					fn:   fn,
				}
				val = t
			} else {
				panic(fmt.Errorf("object doesn't has member `%s`", name))
			}
		}
		stk.Push(val)
		return
	case typeClassPtr:
		o := v.(*Class)
		val, ok := o.Fns[name]
		if !ok {
			panic(fmt.Errorf("class doesn't has method `%s`", name))
		}
		stk.Push(val)
		return
	}

	obj := reflect.ValueOf(v)
	switch {
	case obj.Kind() == reflect.Map:
		m := obj.MapIndex(reflect.ValueOf(name))
		if m.IsValid() {
			stk.Push(m.Interface())
		} else {
			panic(fmt.Errorf("member `%s` not found", name))
		}
	default:
		name = strings.Title(name)
		m := obj.MethodByName(name)
		if m.IsValid() {
			if qlang.AutoCall[t] && m.Type().NumIn() == 0 {
				out := m.Call(nil)
				stk.PushRet(out)
				return
			}
		} else {
			m = reflect.Indirect(obj).FieldByName(name)
			if !m.IsValid() {
				panic(fmt.Errorf("type `%v` doesn't has member `%s`", obj.Type(), name))
			}
		}
		stk.Push(m.Interface())
	}
}

func (p *iMemberRef) ToVar() Instr {
	return &iMemberVar{p.name}
}

// MemberRef returns a MemberRef instruction.
//
func MemberRef(name string) Instr {
	return &iMemberRef{name}
}

// -----------------------------------------------------------------------------
// MemberVar

type iMemberVar struct {
	name string
}

func (p *iMemberVar) Exec(stk *Stack, ctx *Context) {

	v, ok := stk.Pop()
	if !ok {
		panic(ErrRefWithoutObject)
	}

	stk.Push(&qlang.DataIndex{Data: v, Index: p.name})
}

// MemberVar returns a MemberVar instruction.
//
func MemberVar(name string) Instr {
	return &iMemberVar{name}
}

// -----------------------------------------------------------------------------
