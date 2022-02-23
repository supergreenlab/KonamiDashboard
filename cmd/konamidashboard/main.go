// Simple program that displays the state of the specified joystick
//
//     go run joysticktest.go 2
// displays state of joystick id 2
package main

import (
	log "github.com/sirupsen/logrus"
	"fmt"
	"github.com/nsf/termbox-go"
	"github.com/supergreenlab/konamidashboard/internal"
	"os"
	"strconv"
	"time"
	"os/exec"
)

const (
	VERT_STICK = 1
	HOR_STICK = 0
	A_BTN = 0
	B_BTN = 2
)

type Step interface {
	StepMatch(js joystick.Joystick, jinfo joystick.State) bool
}

type EmptyStep struct {
}

func (step EmptyStep) StepMatch(js joystick.Joystick, jinfo joystick.State) bool {
	for button := 0; button < js.ButtonCount(); button++ {
		if jinfo.Buttons&(1<<uint32(button)) != 0 {
			return false
		}
	}

	for axis := 0; axis < js.AxisCount(); axis++ {
		if jinfo.AxisData[axis] != 0 {
			return false
		}
	}

	return true
}

type JoystickStep struct {
	axisId int
	positiv bool
}

func (step JoystickStep) StepMatch(js joystick.Joystick, jinfo joystick.State) bool {
	for button := 0; button < js.ButtonCount(); button++ {
		if jinfo.Buttons&(1<<uint32(button)) != 0 {
			return false
		}
	}

	if jinfo.AxisData[step.axisId] == 0 {
		return false
	}
	return (jinfo.AxisData[step.axisId] > 0) == step.positiv
}

type ButtonStep struct {
	buttonId int
}

func (step ButtonStep) StepMatch(js joystick.Joystick, jinfo joystick.State) bool {
	return jinfo.Buttons&(1<<uint32(step.buttonId)) != 0
}

type Sequence struct {
	steps []Step
	matching bool
	current int

	status string
}

func (seq *Sequence) ReadJoystick(js joystick.Joystick, jinfo joystick.State) {
	sm := seq.steps[seq.current].StepMatch(js, jinfo)
	if !sm && seq.matching {
		seq.current += 1
		seq.status = fmt.Sprintf("Switching to next step: %d", seq.current)
		seq.matching = false
		if seq.current == len(seq.steps) {
			seq.status = fmt.Sprintf("TRIGGER KONAMI CODE!!!")
			seq.current = 0
			go triggerKonami()
		}
		return
	}
	if sm && !seq.matching {
		seq.status = fmt.Sprintf("Matching step: %d", seq.current)
		seq.matching = true
		return
	}
	if !sm && !seq.matching {
		if seq.current != 0 {
			seq.status = fmt.Sprintf("Cancel konami code at step: %d", seq.current)
		}
		seq.current = 0
		return
	}
}

var konamiSequence = &Sequence{
	steps: []Step{
		JoystickStep{
			axisId: VERT_STICK,
			positiv: false,
		},
		EmptyStep{},
		JoystickStep{
			axisId: VERT_STICK,
			positiv: false,
		},
		EmptyStep{},
		JoystickStep{
			axisId: VERT_STICK,
			positiv: true,
		},
		EmptyStep{},
		JoystickStep{
			axisId: VERT_STICK,
			positiv: true,
		},
		EmptyStep{},

		JoystickStep{
			axisId: HOR_STICK,
			positiv: false,
		},
		EmptyStep{},
		JoystickStep{
			axisId: HOR_STICK,
			positiv: true,
		},
		EmptyStep{},
		JoystickStep{
			axisId: HOR_STICK,
			positiv: false,
		},
		EmptyStep{},
		JoystickStep{
			axisId: HOR_STICK,
			positiv: true,
		},
		EmptyStep{},

		ButtonStep{
			buttonId: B_BTN,
		},
		EmptyStep{},
		ButtonStep{
			buttonId: A_BTN,
		},
	},
	current: 0,
}

func triggerKonami() {
	cmd, err := exec.Command("/bin/bash", "-c", "DISPLAY=:0 chromium --kiosk --new-window https://hq.supergreenlab.com/").Output()
	if err != nil {
		log.Errorf("error %s", err)
	}
	output := string(cmd)
	log.Info(output)

}

func printAt(x, y int, s string) {
	for _, r := range s {
		termbox.SetCell(x, y, r, termbox.ColorDefault, termbox.ColorDefault)
		x++
	}
}

func readJoystick(js joystick.Joystick) {
	jinfo, err := js.Read()

	if err != nil {
		printAt(1, 5, "Error: "+err.Error())
		return
	}

	printAt(1, 5, "Buttons:")
	for button := 0; button < js.ButtonCount(); button++ {
		if jinfo.Buttons&(1<<uint32(button)) != 0 {
			printAt(10+button, 5, "X")
		} else {
			printAt(10+button, 5, ".")
		}
	}

	for axis := 0; axis < js.AxisCount(); axis++ {
		printAt(1, axis+7, fmt.Sprintf("Axis %2d Value: %7d", axis, jinfo.AxisData[axis]))
	}

	konamiSequence.ReadJoystick(js, jinfo)
	printAt(1, 20, konamiSequence.status)
	return
}

func main() {

	jsid := 0
	if len(os.Args) > 1 {
		i, err := strconv.Atoi(os.Args[1])
		if err != nil {
			fmt.Println(err)
			return
		}
		jsid = i
	}

	js, jserr := joystick.Open(jsid)

	if jserr != nil {
		fmt.Println(jserr)
		return
	}

	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()

	eventQueue := make(chan termbox.Event)
	go func() {
		for {
			eventQueue <- termbox.PollEvent()
		}
	}()

	ticker := time.NewTicker(time.Millisecond * 40)

	for doQuit := false; !doQuit; {
		select {
		case ev := <-eventQueue:
			if ev.Type == termbox.EventKey {
				if ev.Ch == 'q' {
					doQuit = true
				}
			}
			if ev.Type == termbox.EventResize {
				termbox.Flush()
			}

		case <-ticker.C:
			printAt(1, 0, "-- Press 'q' to Exit --")
			printAt(1, 1, fmt.Sprintf("Joystick Name: %s", js.Name()))
			printAt(1, 2, fmt.Sprintf("   Axis Count: %d", js.AxisCount()))
			printAt(1, 3, fmt.Sprintf(" Button Count: %d", js.ButtonCount()))
			readJoystick(js)
			termbox.Flush()
		}
	}
}
