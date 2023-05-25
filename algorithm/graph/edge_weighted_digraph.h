#ifndef ALGORITHM_EDGE_WEIGHTED_DIGRAPH_H
#define ALGORITHM_EDGE_WEIGHTED_DIGRAPH_H

#include <cassert>
#include <iomanip>
#include <iostream>
#include <type_traits>
#include <vector>

namespace algorithm {

class DirectedEdge {
friend std::ostream& operator<<(std::ostream&, const DirectedEdge&);
    struct Forbidden {};
public:
    // default ctor for container of STL
    DirectedEdge(Forbidden = Forbidden());

    DirectedEdge(int u, int v, double weight);

    DirectedEdge(const DirectedEdge& edge);

    DirectedEdge& operator=(const DirectedEdge& edge);

    double Weight() const {
        return weight_;
    }

    int From() const {
        return from_;
    }

    int To() const {
        return to_;
    }

private:
    int from_;
    int to_;
    double weight_;
};

std::ostream& operator<<(std::ostream& os, const DirectedEdge& edge);

class EdgeWeightedDigraph {
friend std::ostream& operator<<(std::ostream&, const EdgeWeightedDigraph&);
public:
    EdgeWeightedDigraph();

    EdgeWeightedDigraph(int vNum);

    EdgeWeightedDigraph(std::istream& in);

    EdgeWeightedDigraph(const EdgeWeightedDigraph& graph);

    EdgeWeightedDigraph(EdgeWeightedDigraph&& graph);

    EdgeWeightedDigraph& operator=(const EdgeWeightedDigraph& graph);

    EdgeWeightedDigraph& operator=(EdgeWeightedDigraph&& graph);

    int VertexNum() const {
        return vNum_;
    }

    int EdgeNum() const {
        return eNum_;
    }

    void AddEdge(const DirectedEdge& edge);

    const std::vector<DirectedEdge>& Adjacent(int v) const {
        assert(v < vNum_);
        return adj_[v];
    }

    const std::vector<std::vector<DirectedEdge>>& Edges() const {
        return adj_;
    }

private:
    void CheckInputStream(std::istream& in, bool checkEof) const;

    int vNum_;
    int eNum_;
    std::vector<std::vector<DirectedEdge>> adj_;
};

std::ostream& operator<<(std::ostream& os, const EdgeWeightedDigraph& graph);

} 

#endif