// Package fomod parses FOMOD ModuleConfig.xml installers and evaluates their
// install steps so a caller can drive an interactive wizard and resolve a
// final source -> destination file list.
package fomod

import "encoding/xml"

// Config is the root <config> element of ModuleConfig.xml.
type Config struct {
	XMLName              xml.Name             `xml:"config"`
	ModuleName           string               `xml:"moduleName"`
	ModuleDependencies    *Dependencies        `xml:"moduleDependencies"`
	RequiredInstallFiles  FileList             `xml:"requiredInstallFiles"`
	InstallSteps          InstallSteps         `xml:"installSteps"`
	ConditionalFileInstalls ConditionalFileInstalls `xml:"conditionalFileInstalls"`
}

// InstallSteps is the <installSteps> element, with its ordering attribute.
type InstallSteps struct {
	Order string        `xml:"order,attr"` // "Explicit" (default) | "Ascending" | "Descending"
	Steps []InstallStep `xml:"installStep"`
}

// InstallStep is a single <installStep>.
type InstallStep struct {
	Name              string        `xml:"name,attr"`
	Visible           *Dependencies `xml:"visible"`
	OptionalFileGroups Groups       `xml:"optionalFileGroups"`
}

// Groups is the <optionalFileGroups> element.
type Groups struct {
	Order  string  `xml:"order,attr"`
	Groups []Group `xml:"group"`
}

// GroupType mirrors the FOMOD <group type="..."> values.
type GroupType string

const (
	GroupSelectAtLeastOne GroupType = "SelectAtLeastOne"
	GroupSelectAtMostOne  GroupType = "SelectAtMostOne"
	GroupSelectExactlyOne GroupType = "SelectExactlyOne"
	GroupSelectAll        GroupType = "SelectAll"
	GroupSelectAny        GroupType = "SelectAny"
)

// Group is a single <group> within <optionalFileGroups>.
type Group struct {
	Name    string    `xml:"name,attr"`
	Type    GroupType `xml:"type,attr"`
	Plugins Plugins   `xml:"plugins"`
}

// Plugins is the <plugins> element.
type Plugins struct {
	Order   string   `xml:"order,attr"`
	Plugins []Plugin `xml:"plugin"`
}

// Plugin is a single <plugin> (a selectable option).
type Plugin struct {
	Name           string          `xml:"name,attr"`
	Description    string          `xml:"description"`
	Image          *PluginImage    `xml:"image"`
	Files          FileList        `xml:"files"`
	ConditionFlags ConditionFlags  `xml:"conditionFlags"`
	TypeDescriptor TypeDescriptor  `xml:"typeDescriptor"`
}

// PluginImage is the optional <image path="..."/> for a plugin.
type PluginImage struct {
	Path string `xml:"path,attr"`
}

// PluginType mirrors the FOMOD <type name="..."> values.
type PluginType string

const (
	PluginRequired       PluginType = "Required"
	PluginOptional       PluginType = "Optional"
	PluginRecommended    PluginType = "Recommended"
	PluginNotUsable      PluginType = "NotUsable"
	PluginCouldBeUsable  PluginType = "CouldBeUsable"
)

// TypeDescriptor describes a plugin's default selectability, either as a
// fixed <type name="..."/> or a conditional <dependencyType>.
type TypeDescriptor struct {
	Type           *PluginTypeElem `xml:"type"`
	DependencyType *DependencyType `xml:"dependencyType"`
}

// PluginTypeElem is the plain <type name="..."/> form.
type PluginTypeElem struct {
	Name PluginType `xml:"name,attr"`
}

// DependencyType is the conditional <dependencyType> form.
type DependencyType struct {
	DefaultType PluginTypeElem `xml:"defaultType"`
	Patterns    []TypePattern  `xml:"patterns>pattern"`
}

// TypePattern is a <pattern> within a <dependencyType>'s <patterns>.
type TypePattern struct {
	Dependencies Dependencies   `xml:"dependencies"`
	Type         PluginTypeElem `xml:"type"`
}

// ConditionFlags is the <conditionFlags> element, listing flags a plugin
// selection sets when chosen.
type ConditionFlags struct {
	Flags []ConditionFlag `xml:"flag"`
}

// ConditionFlag is a single <flag name="...">value</flag>.
type ConditionFlag struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

// Dependencies is a <dependencies>/<visible>/composite-dependency container:
// the operator applies across all child dependency kinds present.
type Dependencies struct {
	Operator             string                `xml:"operator,attr"` // "And" (default) | "Or"
	FileDependencies     []FileDependency      `xml:"fileDependency"`
	FlagDependencies     []FlagDependency      `xml:"flagDependency"`
	GameDependencies     []GameDependency      `xml:"gameDependency"`
	CompositeDependencies []Dependencies       `xml:"dependencies"`
}

// FileDependency checks the state of a real file in the game install.
// v1 evaluates this as always-satisfied (see engine.go).
type FileDependency struct {
	File  string `xml:"file,attr"`
	State string `xml:"state,attr"` // "Active" | "Inactive" | "Missing"
}

// FlagDependency checks a condition flag set by an earlier plugin selection.
type FlagDependency struct {
	Flag  string `xml:"flag,attr"`
	Value string `xml:"value,attr"`
}

// GameDependency checks the host game's version. v1 evaluates this as
// always-satisfied (see engine.go).
type GameDependency struct {
	Version string `xml:"version,attr"`
}

// FileList is a <files>/<requiredInstallFiles> element: a list of file/folder
// entries to install.
type FileList struct {
	Files   []FileItem `xml:"file"`
	Folders []FileItem `xml:"folder"`
}

// FileItem is a single <file> or <folder> entry with source/destination.
type FileItem struct {
	Source      string `xml:"source,attr"`
	Destination string `xml:"destination,attr"`
	Priority    int    `xml:"priority,attr"`
	AlwaysInstall bool `xml:"alwaysInstall,attr"`
}

// ConditionalFileInstalls is the top-level <conditionalFileInstalls> element.
type ConditionalFileInstalls struct {
	Patterns []FilePattern `xml:"patterns>pattern"`
}

// FilePattern is a <pattern> within <conditionalFileInstalls>: install Files
// when Dependencies evaluates true against the final flag state.
type FilePattern struct {
	Dependencies Dependencies `xml:"dependencies"`
	Files        FileList     `xml:"files"`
}
