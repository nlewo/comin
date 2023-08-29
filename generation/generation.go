package generation

import (
	"github.com/nlewo/comin/repository"
	"time"
)

type Generations struct {
	Limit       int
	Generations []Generation `json:"generations"`
}

type Generation struct {
	SwitchOperation     string                      `json:"switch_operation"`
	Status              string                      `json:"status"`
	DeploymentStartedAt time.Time                   `json:"deployment_started_at"`
	DeploymentEndedAt   time.Time                   `json:"deployment_ended_at"`
	RepositoryStatus    repository.RepositoryStatus `json:"repository_status"`
}

func NewGenerations(limit int, generations []Generation) *Generations {
	g := make([]Generation, 0)
	if len(generations) > limit {
		g = append(g, generations[:limit]...)
	} else {
		g = append(g, generations...)
	}
	return &Generations{
		Limit:       limit,
		Generations: g,
	}
}

func (generations *Generations) InsertNewGeneration(generation Generation) {
	g := make([]Generation, 1)
	g[0] = generation
	if len(generations.Generations) > generations.Limit {
		generations.Generations = append(g, generations.Generations[:generations.Limit-1]...)
	} else {
		generations.Generations = append(g, generations.Generations...)
	}
}

func (generations *Generations) GetGenerationAt(index int) Generation {
	return generations.Generations[index]
}

func (generations *Generations) ReplaceGenerationAt(index int, generation Generation) {
	generations.Generations[index] = generation
}
