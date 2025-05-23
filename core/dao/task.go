package dao

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jinzhu/copier"
	"github.com/theckman/yacspin"
	"gopkg.in/yaml.v3"

	core "github.com/alajmo/mani/core"
)

var (
	build_mode = "dev"
)

type Command struct {
	Name    string    `yaml:"name"`
	Desc    string    `yaml:"desc"`
	Shell   string    `yaml:"shell"` // should be in the format: <program> <command flag>, for instance "sh -c", "node -e"
	Cmd     string    `yaml:"cmd"`   // "echo hello world", it should not include the program flag (-c,-e, .etc)
	Task    string    `yaml:"task"`
	TaskRef string    `yaml:"-"` // Keep a reference to the task
	TTY     bool      `yaml:"tty"`
	Env     yaml.Node `yaml:"env"`
	EnvList []string  `yaml:"-"`

	// Internal
	ShellProgram string   `yaml:"-"` // should be in the format: <program>, example: "sh", "node"
	CmdArg       []string `yaml:"-"` // is in the format ["-c echo hello world"] or ["-c", "echo hello world"], it includes the shell flag
}

type Task struct {
	SpecData   Spec
	TargetData Target
	ThemeData  Theme

	Name     string    `yaml:"name"`
	Desc     string    `yaml:"desc"`
	Shell    string    `yaml:"shell"`
	Cmd      string    `yaml:"cmd"`
	Commands []Command `yaml:"commands"`
	EnvList  []string  `yaml:"-"`
	TTY      bool      `yaml:"tty"`

	Env    yaml.Node `yaml:"env"`
	Spec   yaml.Node `yaml:"spec"`
	Target yaml.Node `yaml:"target"`
	Theme  yaml.Node `yaml:"theme"`

	// Internal
	ShellProgram string   `yaml:"-"` // should be in the format: <program>, example: "sh", "node"
	CmdArg       []string `yaml:"-"` // is in the format ["-c echo hello world"] or ["-c", "echo hello world"], it includes the shell flag
	context      string
	contextLine  int
}

func (t *Task) GetContext() string {
	return t.context
}

func (t *Task) GetContextLine() int {
	return t.contextLine
}

// ParseTask parses tasks and builds the correct "AST". Depending on if the data is specified inline,
// or if it is a reference to resource, it will handle them differently.
func (t *Task) ParseTask(config Config, taskErrors *ResourceErrors[Task]) {
	if t.Shell == "" {
		t.Shell = config.Shell
	} else {
		t.Shell = core.FormatShell(t.Shell)
	}

	program, cmdArgs := core.FormatShellString(t.Shell, t.Cmd)
	t.ShellProgram = program
	t.CmdArg = cmdArgs

	for j, cmd := range t.Commands {
		// Task reference
		if cmd.Task != "" {
			cmdRef, err := config.GetCommand(cmd.Task)
			if err != nil {
				taskErrors.Errors = append(taskErrors.Errors, err)
				continue
			}

			t.Commands[j] = *cmdRef
			t.Commands[j].TaskRef = cmd.Task
		}

		if t.Commands[j].Shell == "" {
			t.Commands[j].Shell = DEFAULT_SHELL
		}

		program, cmdArgs := core.FormatShellString(t.Commands[j].Shell, t.Commands[j].Cmd)
		t.Commands[j].ShellProgram = program
		t.Commands[j].CmdArg = cmdArgs
	}

	if len(t.Theme.Content) > 0 {
		// Theme value
		theme := &Theme{}
		err := t.Theme.Decode(theme)
		if err != nil {
			taskErrors.Errors = append(taskErrors.Errors, err)
		} else {
			t.ThemeData = *theme
		}
	} else if t.Theme.Value != "" {
		// Theme reference
		theme, err := config.GetTheme(t.Theme.Value)
		if err != nil {
			taskErrors.Errors = append(taskErrors.Errors, err)
		} else {
			t.ThemeData = *theme
		}
	} else {
		// Default theme
		theme, err := config.GetTheme(DEFAULT_THEME.Name)
		if err != nil {
			taskErrors.Errors = append(taskErrors.Errors, err)
		} else {
			t.ThemeData = *theme
		}
	}

	if len(t.Spec.Content) > 0 {
		// Spec value
		spec := &Spec{}
		err := t.Spec.Decode(spec)

		if err != nil {
			taskErrors.Errors = append(taskErrors.Errors, err)
		} else {
			t.SpecData = *spec
		}
	} else if t.Spec.Value != "" {
		// Spec reference
		spec, err := config.GetSpec(t.Spec.Value)
		if err != nil {
			taskErrors.Errors = append(taskErrors.Errors, err)
		} else {
			t.SpecData = *spec
		}
	} else {
		// Default spec
		spec, err := config.GetSpec(DEFAULT_SPEC.Name)
		if err != nil {
			taskErrors.Errors = append(taskErrors.Errors, err)
		} else {
			t.SpecData = *spec
		}
	}

	if len(t.Target.Content) > 0 {
		// Target value
		target := &Target{}
		err := t.Target.Decode(target)
		if err != nil {
			taskErrors.Errors = append(taskErrors.Errors, err)
		} else {
			t.TargetData = *target
		}
	} else if t.Target.Value != "" {
		// Target reference
		target, err := config.GetTarget(t.Target.Value)
		if err != nil {
			taskErrors.Errors = append(taskErrors.Errors, err)
		} else {
			t.TargetData = *target
		}
	} else {
		// Default target
		target, err := config.GetTarget(DEFAULT_TARGET.Name)
		if err != nil {
			taskErrors.Errors = append(taskErrors.Errors, err)
		} else {
			t.TargetData = *target
		}
	}
}

func TaskSpinner() (yacspin.Spinner, error) {
	var cfg yacspin.Config

	// NOTE: Don't print the spinner in tests since it causes
	// golden files to produce different results.
	if build_mode == "TEST" {
		cfg = yacspin.Config{
			Frequency:       100 * time.Millisecond,
			CharSet:         yacspin.CharSets[9],
			SuffixAutoColon: false,
			Writer:          io.Discard,
		}
	} else {
		cfg = yacspin.Config{
			Frequency:       100 * time.Millisecond,
			CharSet:         yacspin.CharSets[9],
			SuffixAutoColon: false,
			ShowCursor:      true,
		}
	}

	spinner, err := yacspin.New(cfg)

	return *spinner, err
}

func (t Task) GetValue(key string, _ int) string {
	switch key {
	case "Name", "name", "Task", "task":
		return t.Name
	case "Desc", "desc", "Description", "description":
		return t.Desc
	case "Command", "command":
		return t.Cmd
	case "Spec", "spec":
		return t.SpecData.Name
	case "Target", "target":
		return t.TargetData.Name
	}

	return ""
}

func (c *Config) GetTaskList() ([]Task, []ResourceErrors[Task]) {
	var tasks []Task
	count := len(c.Tasks.Content)

	taskErrors := []ResourceErrors[Task]{}
	foundErrors := false
	for i := 0; i < count; i += 2 {
		task := &Task{
			Name:        c.Tasks.Content[i].Value,
			context:     c.Path,
			contextLine: c.Tasks.Content[i].Line,
		}

		// Shorthand definition: example_task: echo 123
		if c.Tasks.Content[i+1].Kind == 8 {
			task.Cmd = c.Tasks.Content[i+1].Value
		} else { // Full definition
			err := c.Tasks.Content[i+1].Decode(task)
			if err != nil {
				foundErrors = true
				taskError := ResourceErrors[Task]{Resource: task, Errors: core.StringsToErrors(err.(*yaml.TypeError).Errors)}
				taskErrors = append(taskErrors, taskError)
				continue
			}
		}

		tasks = append(tasks, *task)
	}

	if foundErrors {
		return tasks, taskErrors
	}

	return tasks, nil
}

func ParseTaskEnv(
	env yaml.Node,
	userEnv []string,
	parentEnv []string,
	configEnv []string,
) ([]string, error) {
	cmdEnv, err := EvaluateEnv(ParseNodeEnv(env))
	if err != nil {
		return []string{}, err
	}

	pEnv, err := EvaluateEnv(parentEnv)
	if err != nil {
		return []string{}, err
	}

	envList := MergeEnvs(userEnv, cmdEnv, pEnv, configEnv)

	return envList, nil
}

func ParseTasksEnv(tasks []Task) {
	for i := range tasks {
		envs, err := ParseTaskEnv(tasks[i].Env, []string{}, []string{}, []string{})
		core.CheckIfError(err)

		tasks[i].EnvList = envs

		for j := range tasks[i].Commands {
			envs, err = ParseTaskEnv(tasks[i].Commands[j].Env, []string{}, []string{}, []string{})
			core.CheckIfError(err)

			tasks[i].Commands[j].EnvList = envs
		}
	}
}

// GetTaskProjects retrieves a filtered list of projects for a given task, applying
// runtime flag overrides and target configurations.
//
// Behavior depends on the provided runtime flags (flags, setFlags) and task target:
//   - If runtime flags are set (Projects, Paths, Tags, etc.), they take precedence
//     and reset the task's target configuration.
//   - If a target is explicitly specified (flags.Target), it loads and applies that
//     target's configuration before applying runtime flag overrides.
//   - If no runtime flags or target are provided, the task's default target data is used.
//
// Filtering priority (highest to lowest):
//  1. Runtime flags (e.g., --projects, --tags, --cwd)
//  2. Explicit target configuration (--target)
//  3. Task's default target data (if no overrides exist)
//
// Returns:
//   - Filtered []Project based on the resolved configuration.
//   - Non-nil error if target resolution or project filtering fails.
func (c Config) GetTaskProjects(
	task *Task,
	flags *core.RunFlags,
	setFlags *core.SetRunFlags,
) ([]Project, error) {
	var err error
	var projects []Project

	// Reset target if any runtime flags are used
	if len(flags.Projects) > 0 ||
		len(flags.Paths) > 0 ||
		len(flags.Tags) > 0 ||
		flags.TagsExpr != "" ||
		flags.Target != "" ||
		setFlags.Cwd ||
		setFlags.All {
		task.TargetData = Target{}
	}

	if flags.Target != "" {
		target, err := c.GetTarget(flags.Target)
		if err != nil {
			return []Project{}, err
		}
		task.TargetData = *target
	}

	if len(flags.Projects) > 0 {
		task.TargetData.Projects = flags.Projects
	}

	if len(flags.Paths) > 0 {
		task.TargetData.Paths = flags.Paths
	}

	if len(flags.Tags) > 0 {
		task.TargetData.Tags = flags.Tags
	}

	if flags.TagsExpr != "" {
		task.TargetData.TagsExpr = flags.TagsExpr
	}

	if setFlags.Cwd {
		task.TargetData.Cwd = flags.Cwd
	}

	if setFlags.All {
		task.TargetData.All = flags.All
	}

	projects, err = c.FilterProjects(
		task.TargetData.Cwd,
		task.TargetData.All,
		task.TargetData.Projects,
		task.TargetData.Paths,
		task.TargetData.Tags,
		task.TargetData.TagsExpr,
	)
	if err != nil {
		return []Project{}, err
	}

	return projects, nil
}

func (c Config) GetTasksByNames(names []string) ([]Task, error) {
	if len(names) == 0 {
		return c.TaskList, nil
	}

	foundTasks := make(map[string]bool)
	for _, t := range names {
		foundTasks[t] = false
	}

	var filteredTasks []Task
	for _, name := range names {
		if foundTasks[name] {
			continue
		}

		for _, task := range c.TaskList {
			if name == task.Name {
				foundTasks[task.Name] = true
				filteredTasks = append(filteredTasks, task)
			}
		}
	}

	nonExistingTasks := []string{}
	for k, v := range foundTasks {
		if !v {
			nonExistingTasks = append(nonExistingTasks, k)
		}
	}

	if len(nonExistingTasks) > 0 {
		return []Task{}, &core.TaskNotFound{Name: nonExistingTasks}
	}

	return filteredTasks, nil
}

func (c Config) GetTaskNames() []string {
	taskNames := []string{}
	for _, task := range c.TaskList {
		taskNames = append(taskNames, task.Name)
	}

	return taskNames
}

func (c Config) GetTaskNameAndDesc() []string {
	taskNames := []string{}
	for _, task := range c.TaskList {
		taskNames = append(taskNames, fmt.Sprintf("%s\t%s", task.Name, task.Desc))
	}

	return taskNames
}

func (c Config) GetTask(name string) (*Task, error) {
	for _, cmd := range c.TaskList {
		if name == cmd.Name {
			return &cmd, nil
		}
	}

	return nil, &core.TaskNotFound{Name: []string{name}}
}

func (c Config) GetCommand(taskName string) (*Command, error) {
	for _, cmd := range c.TaskList {
		if taskName == cmd.Name {
			cmdRef := &Command{
				Name:    cmd.Name,
				Desc:    cmd.Desc,
				EnvList: cmd.EnvList,
				Shell:   cmd.Shell,
				Cmd:     cmd.Cmd,
			}

			return cmdRef, nil
		}
	}

	return nil, &core.TaskNotFound{Name: []string{taskName}}
}

func (t Task) ConvertTaskToCommand() Command {
	cmd := Command{
		Name:         t.Name,
		Desc:         t.Desc,
		EnvList:      t.EnvList,
		Shell:        t.Shell,
		Cmd:          t.Cmd,
		CmdArg:       t.CmdArg,
		ShellProgram: t.ShellProgram,
	}

	return cmd
}

func ParseCmd(
	cmd string,
	runFlags *core.RunFlags,
	setFlags *core.SetRunFlags,
	config *Config,
) ([]Task, []Project, error) {
	task := Task{Name: "output", Cmd: cmd, TTY: runFlags.TTY}
	taskErrors := make([]ResourceErrors[Task], 1)
	task.ParseTask(*config, &taskErrors[0])

	var configErr = ""
	for _, taskError := range taskErrors {
		if len(taskError.Errors) > 0 {
			configErr = fmt.Sprintf("%s%s", configErr, FormatErrors(taskError.Resource, taskError.Errors))
		}
	}
	if configErr != "" {
		core.CheckIfError(errors.New(configErr))
	}

	projects, err := config.GetTaskProjects(&task, runFlags, setFlags)
	if err != nil {
		return nil, nil, err
	}
	core.CheckIfError(err)

	var tasks []Task
	for range projects {
		t := Task{}
		err := copier.Copy(&t, &task)
		core.CheckIfError(err)
		tasks = append(tasks, t)
	}

	if len(projects) == 0 {
		return nil, nil, &core.NoTargets{}
	}

	return tasks, projects, err
}

func ParseSingleTask(
	taskName string,
	runFlags *core.RunFlags,
	setFlags *core.SetRunFlags,
	config *Config,
) ([]Task, []Project, error) {
	task, err := config.GetTask(taskName)
	core.CheckIfError(err)

	projects, err := config.GetTaskProjects(task, runFlags, setFlags)
	core.CheckIfError(err)

	var tasks []Task
	for range projects {
		t := Task{}
		err := copier.Copy(&t, &task)
		core.CheckIfError(err)
		tasks = append(tasks, t)
	}

	if len(projects) == 0 {
		return nil, nil, &core.NoTargets{}
	}

	return tasks, projects, err
}

func ParseManyTasks(
	taskNames []string,
	runFlags *core.RunFlags,
	setFlags *core.SetRunFlags,
	config *Config,
) ([]Task, []Project, error) {
	parentTask := Task{Name: "Tasks", Cmd: "", Commands: []Command{}}
	taskErrors := make([]ResourceErrors[Task], 1)
	parentTask.ParseTask(*config, &taskErrors[0])

	for _, taskName := range taskNames {
		task, err := config.GetTask(taskName)
		core.CheckIfError(err)

		if task.Cmd != "" {
			cmd := task.ConvertTaskToCommand()
			parentTask.Commands = append(parentTask.Commands, cmd)
		} else if len(task.Commands) > 0 {
			parentTask.Commands = append(parentTask.Commands, task.Commands...)
		}
	}

	projects, err := config.GetTaskProjects(&parentTask, runFlags, setFlags)
	var tasks []Task
	for range projects {
		t := Task{}
		err := copier.Copy(&t, &parentTask)
		core.CheckIfError(err)
		tasks = append(tasks, t)
	}

	if len(projects) == 0 {
		return nil, nil, &core.NoTargets{}
	}

	return tasks, projects, err
}
