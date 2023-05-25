#ifndef ALGORITHM_GRAPH_DIJKSTRA_H
#define ALGORITHM_GRAPH_DIJKSTRA_H

#include <queue>
#include <utility>

#include "graph/edge_weighted_digraph.h"

namespace algorithm {

class Dijkstra {
friend std::ostream& operator<<(std::ostream&, const Dijkstra&);
    typedef std::pair<int, double> PriorityQueueElem;
    typedef std::vector<PriorityQueueElem> PriorityQueueContainer;
    struct PriorityQueueCmp {
        bool operator()(const PriorityQueueElem& lhs, 
                        const PriorityQueueElem& rhs) const {
            return lhs.second > rhs.second;
        }
    };
public:
    Dijkstra();

    Dijkstra(const EdgeWeightedDigraph& graph, int start);

    bool HasPathTo(int v) const;

    double DistTo(int v) const {
        assert(v < distance_.size());
        return distance_[v];
    }

    std::vector<DirectedEdge> PathTo(int v) const;

private:
    int start_;
    std::vector<double> distance_;
    std::vector<DirectedEdge> edgeTo_;
};

std::ostream& operator<<(std::ostream& os, const Dijkstra& dijkstra);

}

#endif