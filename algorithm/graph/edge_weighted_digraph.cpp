#include "graph/edge_weighted_digraph.h"

namespace algorithm {

DirectedEdge::DirectedEdge(Forbidden)
    : from_(0), to_(0), weight_(0.0) {}

DirectedEdge::DirectedEdge(int u, int v, double weight)
    : from_(u), to_(v), weight_(weight) {}

DirectedEdge::DirectedEdge(const DirectedEdge& edge)
    : from_(edge.from_), to_(edge.to_), weight_(edge.weight_) {}

DirectedEdge& DirectedEdge::operator=(const DirectedEdge& edge) {
    this->from_ = edge.from_;
    this->to_ = edge.to_;
    this->weight_ = edge.weight_;
    return *this;
}

std::ostream& operator<<(std::ostream& os, const DirectedEdge& edge) {
    os << "{ " << edge.from_ << "->" << edge.to_ 
       << " " << std::setprecision(3) << edge.weight_ << " }";
    return os;
}

EdgeWeightedDigraph::EdgeWeightedDigraph()
    : vNum_(0), eNum_(0) {}

EdgeWeightedDigraph::EdgeWeightedDigraph(int vNum)
    : vNum_(vNum), eNum_(0), adj_(vNum, std::vector<DirectedEdge>{}) {}

EdgeWeightedDigraph::EdgeWeightedDigraph(const EdgeWeightedDigraph& graph)
    : vNum_(graph.vNum_), eNum_(graph.eNum_), adj_(graph.adj_) {}

EdgeWeightedDigraph::EdgeWeightedDigraph(EdgeWeightedDigraph&& graph)
    : vNum_(graph.vNum_), eNum_(graph.eNum_), adj_(std::move(graph.adj_)) {
    graph.vNum_ = 0;
    graph.eNum_ = 0;
}

EdgeWeightedDigraph& EdgeWeightedDigraph::operator=(const EdgeWeightedDigraph& graph) {
    this->vNum_ = graph.vNum_;
    this->eNum_ = graph.eNum_;
    this->adj_ = graph.adj_;
    return *this;
}

EdgeWeightedDigraph& EdgeWeightedDigraph::operator=(EdgeWeightedDigraph&& graph) {
    this->vNum_ = graph.vNum_;
    this->eNum_ = graph.eNum_;
    this->adj_ = std::move(graph.adj_);
    graph.vNum_ = 0;
    graph.eNum_ = 0;
    return *this;
}

void EdgeWeightedDigraph::AddEdge(const DirectedEdge& edge) {
    assert(edge.From() < vNum_);
    assert(edge.To() < vNum_);
    adj_[edge.From()].push_back(edge);
    ++eNum_;
}

void EdgeWeightedDigraph::CheckInputStream(std::istream& in, bool checkEof) const {
    if (checkEof) {
        assert(!in.eof());
    }
    assert(!in.bad());
}

EdgeWeightedDigraph::EdgeWeightedDigraph(std::istream& in) 
    : vNum_(0), eNum_(0) {
    CheckInputStream(in, true);
    in >> vNum_;
    CheckInputStream(in, true);
    assert(vNum_ > 0);
    adj_.resize(vNum_, std::vector<DirectedEdge>{});
    in >> eNum_;
    assert(eNum_ >= 0);
    if (eNum_ == 0) {
        return;
    }
    for (int i = 0; i < eNum_; i++) {
        CheckInputStream(in, i + 1 < eNum_);
        int from = -1, to = -1;
        double weight = 0.0;
        in >> from >> to >> weight;
        DirectedEdge edge(from, to, weight);
        adj_[from].push_back(edge);
    }
}

std::ostream& operator<<(std::ostream& os, const EdgeWeightedDigraph& graph) {
    os << "{ vNum:" << graph.vNum_ << ",eNum:" << graph.eNum_ << ",edges:";
    for (const std::vector<DirectedEdge>& edges : graph.adj_) {
        for (auto it = edges.begin(); it != edges.end(); ++it) {
            os << *it;
            if (it + 1 != edges.end()) {
                os << " ";
            }
        }
    }
    os << " }";
    return os;
}

}