package plugin

type Plugin struct {
	Name     string     `yaml:"name"`
	Version  string     `yaml:"version"`
	Detect   Detect     `yaml:"detect"`
	Commands []Command  `yaml:"commands"`
	Env      []EnvCheck `yaml:"env"`
	Source   string     `yaml:"-"`
}

type Detect struct {
	Files []string `yaml:"files"`
	Dirs  []string `yaml:"dirs"`
}

type Command struct {
	Name        string     `yaml:"name"`
	Run         string     `yaml:"run"`
	Description string     `yaml:"description,omitempty"`
	Risk        string     `yaml:"risk,omitempty"`
	Confirm     bool       `yaml:"confirm,omitempty"`
	Source      string     `yaml:"-"`
	Checks      []EnvCheck `yaml:"-"`
	Confidence  int        `yaml:"-"`
}

type EnvCheck struct {
	Name     string `yaml:"name"`
	Check    string `yaml:"check"`
	Required bool   `yaml:"required"`
}
