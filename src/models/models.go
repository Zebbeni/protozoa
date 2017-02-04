package models

import (
	"fmt"
	"math/rand"
)

const NUM_PROTISTS = 100
const NUM_CYCLES = 1000
const MAX_PARAM = 100

type Nucleotide struct {
	IsAction bool
	Index    int
	Arg      int
}

type Environment struct {
	Temperature int // temperature
	BadWeather  int // # days of bad weather
	GoodWeather int // # days of good weather
	NumDead     int // number of dead protazoa
}

func (e *Environment) UpdateEnvironment() {
	e.Temperature += rand.Intn(21) - 10
	if e.Temperature > 100 {
		e.Temperature = 100
	} else if e.Temperature < 0 {
		e.Temperature = 0
	}

	if e.Temperature > 75 || e.Temperature < 25 {
		e.BadWeather++
	} else {
		e.GoodWeather++
	}
	fmt.Println("Temp:", e.Temperature, "degrees")
}

type Protist struct {
	Id          int
	Health      int
	Food        int
	Days_lived  int
	Covered     bool
	Alive       bool
	Dna         []Nucleotide
	Action      func()
	Environment *Environment
}

func actionCover(p *Protist, arg int) {
	fmt.Print(" Taking cover. ")
	p.Covered = true
}

func actionUncover(p *Protist, arg int) {
	fmt.Print(" Leaving cover. ")
	p.Covered = false
}

func actionEat(p *Protist, arg int) {
	fmt.Print(" Eating. ")
	p.Food += 2
}

func isHealthAbove(p *Protist, arg int) bool {
	if p.Health > arg {
		fmt.Print(" Feels good. Health > ", arg, ". ")
	} else {
		fmt.Print(" Feels bad. Health <= ", arg, ". ")
	}
	return p.Health > arg
}

func isFoodAbove(p *Protist, arg int) bool {
	if p.Food > arg {
		fmt.Print(" Feels full. Food > ", arg, ". ")
	} else {
		fmt.Print(" Feels hungry. Food <= ", arg, ". ")
	}
	return p.Food > arg
}

func isCold(p *Protist, arg int) bool {
	if p.Environment.Temperature < arg {
		fmt.Print(" Feels cold. Temp < ", arg, ". ")
	} else {
		fmt.Print(" Doesn't feel cold. Temp >= ", arg, ". ")
	}
	return p.Environment.Temperature < arg
}

func isHot(p *Protist, arg int) bool {
	if p.Environment.Temperature > arg {
		fmt.Print(" Feels hot. Temp above ", arg, ". ")
	} else {
		fmt.Print(" Doesn't feel hot. Temp below ", arg, ". ")
	}
	return p.Environment.Temperature > arg
}

func (p *Protist) Update() {
	fmt.Println("\nhealth:", p.Health, "food:", p.Food, "covered:", p.Covered)
	if p.Environment.Temperature < 25 && p.Covered == false {
		fmt.Println("models.Protist", p.Id, "is freezing to death")
		p.Health -= 2
	} else if p.Environment.Temperature > 75 && p.Covered == true {
		fmt.Println("models.Protist", p.Id, "is dying of heat")
		p.Health -= 2
	} else {
		p.Health++
	}

	p.Food--
	if p.Food < 0 {
		p.Food = 0
	} else if p.Food > 100 {
		p.Food = 100
	}
	if p.Food < p.Health {
		p.Health = p.Food
	}

	p.Days_lived++

	if p.Health <= 0 {
		p.Alive = false
		p.Environment.NumDead++
		fmt.Println("models.Protist", p.Id, "is dead")
	} else if p.Health > 100 {
		p.Health = 100
	}
}

func (p *Protist) DoCycle() {
	if p.Alive {
		fmt.Print("\nmodels.Protist ", p.Id, " ")
		p.Action() //do models.Protist Actions
		p.Update()
	}
}

func generateNucleotide(isAct bool) Nucleotide {
	numOptions := len(conditions)

	if isAct {
		numOptions = len(actions)
	}
	idx := rand.Intn(numOptions)
	param := rand.Intn(MAX_PARAM)
	newNucleotide := Nucleotide{IsAction: isAct, Index: idx, Arg: param}
	return newNucleotide
}

func (p *Protist) GenerateDNA() {
	p.Dna = []Nucleotide{generateNucleotide(false), generateNucleotide(true), generateNucleotide(true)}
}

func (p *Protist) GenerateActionFromDNA() {
	cond := p.Dna[0]
	action1 := p.Dna[1]
	action2 := p.Dna[2]
	p.Action = func() {
		if p.Alive {
			if conditions[cond.Index](p, cond.Arg) {
				actions[action1.Index](p, action1.Arg)
			} else {
				actions[action2.Index](p, action2.Arg)
			}
		}
	}
}

var conditions = []func(*Protist, int) bool{isHealthAbove, isFoodAbove, isCold, isHot}
var actions = []func(*Protist, int){actionCover, actionUncover, actionEat}
