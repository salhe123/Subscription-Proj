package commitplan

import "context"

type Mutation interface{}

type Plan struct {
	mutations []Mutation
}

func NewPlan() *Plan {
	return &Plan{}
}

func (p *Plan) Add(mut Mutation) {
	if mut != nil {
		p.mutations = append(p.mutations, mut)
	}
}

func (p *Plan) Mutations() []Mutation {
	return p.mutations
}

func (p *Plan) IsEmpty() bool {
	return len(p.mutations) == 0
}

type Committer interface {
	Apply(ctx context.Context, plan *Plan) error
}
