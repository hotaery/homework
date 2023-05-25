#include "graph/dijkstra.h"

#include <algorithm>
#include <cfloat>
#include <unordered_set>

namespace algorithm {

Dijkstra::Dijkstra(const EdgeWeightedDigraph& graph, int start) 
    : start_(start) {
    int vNum = graph.VertexNum();
    assert(start < vNum);
    distance_.resize(vNum, DBL_MAX);
    edgeTo_.resize(vNum);
    distance_[start] = 0.0;

    std::unordered_set<int> visited;
    while (true) {
        int v = -1;
        for (int i = 0; i < vNum; i++) {
            if (visited.count(i)) {
                continue;
            }
            
            if (distance_[i] < DBL_MAX && (v == -1 || distance_[i] < distance_[v])) {
                v = i;
            }
        }
        if (v == -1) {
            break;
        }
        visited.insert(v);

        const std::vector<DirectedEdge>& adj = graph.Adjacent(v);
        for (const DirectedEdge& edge : adj) {
            int to = edge.To();
            if (distance_[to] > distance_[v] + edge.Weight()) {
                distance_[to] = distance_[v] + edge.Weight();
                edgeTo_[to] = edge;
            }
        }
    }
}

bool Dijkstra::HasPathTo(int v) const {
    assert(v < distance_.size());
    return distance_[v] < DBL_MAX;
}

std::vector<DirectedEdge> Dijkstra::PathTo(int v) const {
    assert(v < distance_.size());
    std::vector<DirectedEdge> path;
    if (!(distance_[v] < DBL_MAX) || v == start_) {
        return path;
    }

    do {
        path.push_back(edgeTo_[v]);
        v = edgeTo_[v].From();
    } while (v != start_);

    std::reverse(path.begin(), path.end());
    return path;
}

}