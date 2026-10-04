package config

import (
	"flag"
	"strings"
)

// Register adds a flag to fs for every setting, --<table>-<key>, and its
// earlier names (Spec.FlagAliases). The flags fill fl.Values as they are
// parsed. It returns the aliases' names, which help leaves out
// (PrintDefaults).
func (fl *Flags) Register(fs *flag.FlagSet) (hidden map[string]bool) {
	hidden = map[string]bool{}
	for _, sp := range Specs {
		usage := sp.Help + " (default: " + defaultsText(sp) + ")"
		fs.Var(&flagValue{fl: fl, sp: sp, name: sp.Flag}, strings.TrimPrefix(sp.Flag, "--"), usage)
		for _, a := range sp.FlagAliases {
			name := strings.TrimPrefix(a, "--")
			fs.Var(&flagValue{fl: fl, sp: sp, name: a}, name, "earlier name of "+sp.Flag)
			hidden[name] = true
		}
	}
	return hidden
}

// PrintDefaults prints fs's help, leaving out the hidden flags.
func PrintDefaults(fs *flag.FlagSet, hidden map[string]bool) {
	shown := flag.NewFlagSet(fs.Name(), flag.ContinueOnError)
	shown.SetOutput(fs.Output())
	fs.VisitAll(func(f *flag.Flag) {
		if !hidden[f.Name] {
			shown.Var(f.Value, f.Name, f.Usage)
		}
	})
	shown.PrintDefaults()
}

// defaultsText is where a setting comes from when its flag is not given.
func defaultsText(sp Spec) string {
	var b strings.Builder
	if sp.Env != "" {
		b.WriteString("$" + sp.Env + ", else ")
	}
	b.WriteString(sp.Key + " in the config file, else " + sp.Default)
	return b.String()
}

// flagValue is one setting's flag: it records what was given in Flags.
type flagValue struct {
	fl   *Flags
	sp   Spec
	name string
}

func (v *flagValue) String() string { return "" }

// Set records the value; an earlier name never overrides the flag's own
// name, whatever their order.
func (v *flagValue) Set(text string) error {
	if got, ok := v.fl.Values[v.sp.Key]; ok && got.Name == v.sp.Flag && v.name != v.sp.Flag {
		return nil
	}
	v.fl.Set(v.sp.Key, v.name, text)
	return nil
}

// IsBoolFlag lets an on/off setting's flag stand alone, meaning true.
func (v *flagValue) IsBoolFlag() bool { return v.sp.Kind == KindBool }
