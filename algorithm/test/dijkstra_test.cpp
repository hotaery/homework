#include <fstream>
#include "graph/dijkstra.h"

int main(int argc, char* argv[]) {
    const char* file = "resources/tinyEWD.txt";
    if (argc > 1) {
        file = argv[1];
    }

    std::ifstream in(file);
    algorithm::EdgeWeightedDigraph graph(in);
    std::cout << graph << std::endl;
    algorithm::Dijkstra dijkstra(graph, 0);

    for (int i = 0; i < graph.VertexNum(); i++) {
        std::cout << 0 << " to " << i;
        if (dijkstra.HasPathTo(i)) {
            std::cout << std::setprecision(3) << " (" << dijkstra.DistTo(i) << ")";
            std::vector<algorithm::DirectedEdge> path = dijkstra.PathTo(i);
            for (const algorithm::DirectedEdge& edge : path) {
                std::cout << edge << " ";
            }
        } else {
            std::cout << " (NAN)";
        }
        std::cout << std::endl;
    }

    return 0;
}