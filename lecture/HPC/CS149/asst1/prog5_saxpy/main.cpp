#include <stdio.h>
#include <algorithm>
#include <thread>
#include <cstdlib>
#include <cstdint>
#include <vector>
#include <immintrin.h>

#include "CycleTimer.h"
#include "saxpy_ispc.h"

extern void saxpySerial(int N, float a, float* X, float* Y, float* result);

static void saxpyAVX2Range(int start,
                           int end,
                           float scale,
                           const float* X,
                           const float* Y,
                           float* result) {
    __m256 scaleVec = _mm256_set1_ps(scale);
    int i = start;

    while (i < end && ((reinterpret_cast<uintptr_t>(result + i) & 31) != 0)) {
        result[i] = scale * X[i] + Y[i];
        ++i;
    }

    for (; i + 8 <= end; i += 8) {
        __m256 xVec = _mm256_load_ps(X + i);
        __m256 yVec = _mm256_load_ps(Y + i);
        __m256 outVec = _mm256_fmadd_ps(scaleVec, xVec, yVec);
        // 一种优化，表示这里的结果不需要被缓存，直接写回内存，避免了缓存污染
        _mm256_stream_ps(result + i, outVec);
    }

    for (; i < end; ++i) {
        result[i] = scale * X[i] + Y[i];
    }
}

static void saxpyAVX2(int N, float scale, const float* X, const float* Y, float* result) {
    saxpyAVX2Range(0, N, scale, X, Y, result);
    _mm_sfence();
}

static void saxpyAVX2Threads(int threads,
                             int N,
                             float scale,
                             const float* X,
                             const float* Y,
                             float* result) {
    int threadCount = std::max(1, threads);
    int blockSize = (N + threadCount - 1) / threadCount;
    std::vector<std::thread> workers;
    workers.reserve(threadCount);

    for (int threadIndex = 0; threadIndex < threadCount; ++threadIndex) {
        int start = threadIndex * blockSize;
        int end = std::min(N, start + blockSize);
        if (start >= end) {
            break;
        }

        workers.emplace_back([=]() {
            saxpyAVX2Range(start, end, scale, X, Y, result);
        });
    }

    for (std::thread& worker : workers) {
        worker.join();
    }

    _mm_sfence();
}


// return GB/s
static float
toBW(int bytes, float sec) {
    return static_cast<float>(bytes) / (1024. * 1024. * 1024.) / sec;
}

static float
toGFLOPS(int ops, float sec) {
    return static_cast<float>(ops) / 1e9 / sec;
}

static void verifyResult(int N, float* result, float* gold) {
    for (int i=0; i<N; i++) {
        if (result[i] != gold[i]) {
            printf("Error: [%d] Got %f expected %f\n", i, result[i], gold[i]);
        }
    }
}

using namespace ispc;


int main() {

    const unsigned int N = 20 * 1000 * 1000; // 20 M element vectors (~80 MB)
    const unsigned int TOTAL_BYTES = 4 * N * sizeof(float);
    const unsigned int TOTAL_FLOPS = 2 * N;

    float scale = 2.f;

    float* arrayX = (float*)std::aligned_alloc(64, N * sizeof(float));
    float* arrayY = (float*)std::aligned_alloc(64, N * sizeof(float));
    float* resultSerial = (float*)std::aligned_alloc(64, N * sizeof(float));
    float* resultISPC = (float*)std::aligned_alloc(64, N * sizeof(float));
    float* resultTasks = (float*)std::aligned_alloc(64, N * sizeof(float));
    float* resultAVX2 = (float*)std::aligned_alloc(64, N * sizeof(float));
    float* resultAVX2Threads = (float*)std::aligned_alloc(64, N * sizeof(float));

    // float* arrayX = new float[N];
    // float* arrayY = new float[N];
    // float* resultSerial = new float[N];
    // float* resultISPC = new float[N];
    // float* resultTasks = new float[N];

    // initialize array values
    for (unsigned int i=0; i<N; i++)
    {
        arrayX[i] = i;
        arrayY[i] = i;
        resultSerial[i] = 0.f;
        resultISPC[i] = 0.f;
        resultTasks[i] = 0.f;
        resultAVX2[i] = 0.f;
        resultAVX2Threads[i] = 0.f;
    }

    //
    // Run the serial implementation. Repeat three times for robust
    // timing.
    //
    double minSerial = 1e30;
    for (int i = 0; i < 3; ++i) {
        double startTime =CycleTimer::currentSeconds();
        saxpySerial(N, scale, arrayX, arrayY, resultSerial);
        double endTime = CycleTimer::currentSeconds();
        minSerial = std::min(minSerial, endTime - startTime);
    }

// printf("[saxpy serial]:\t\t[%.3f] ms\t[%.3f] GB/s\t[%.3f] GFLOPS\n",
    //       minSerial * 1000,
    //       toBW(TOTAL_BYTES, minSerial),
    //       toGFLOPS(TOTAL_FLOPS, minSerial));

    //
    // Run the ISPC (single core) implementation
    //
    double minISPC = 1e30;
    for (int i = 0; i < 3; ++i) {
        double startTime = CycleTimer::currentSeconds();
        saxpy_ispc(N, scale, arrayX, arrayY, resultISPC);
        double endTime = CycleTimer::currentSeconds();
        minISPC = std::min(minISPC, endTime - startTime);
    }

    verifyResult(N, resultISPC, resultSerial);

    printf("[saxpy ispc]:\t\t[%.3f] ms\t[%.3f] GB/s\t[%.3f] GFLOPS\n",
           minISPC * 1000,
           toBW(TOTAL_BYTES, minISPC),
           toGFLOPS(TOTAL_FLOPS, minISPC));

    double minAVX2 = 1e30;
    for (int i = 0; i < 3; ++i) {
        double startTime = CycleTimer::currentSeconds();
        saxpyAVX2(N, scale, arrayX, arrayY, resultAVX2);
        double endTime = CycleTimer::currentSeconds();
        minAVX2 = std::min(minAVX2, endTime - startTime);
    }

    verifyResult(N, resultAVX2, resultSerial);

    printf("[saxpy avx2]:\t\t[%.3f] ms\t[%.3f] GB/s\t[%.3f] GFLOPS\n",
           minAVX2 * 1000,
           toBW(TOTAL_BYTES, minAVX2),
           toGFLOPS(TOTAL_FLOPS, minAVX2));

    //
    // Run the ISPC (multi-core) implementation
    //
    int numCores = std::thread::hardware_concurrency();
    if (N % numCores != 0) {
        numCores = numCores - N % numCores; // reduce to a factor of N
    }
    printf("Running with %d tasks\n", numCores);
    int threads[] = {1, 2, 4, 8, 16, 32, 64, 128};
    int threadCount = sizeof(threads) / sizeof(threads[0]);
    for (int j = 0; j < threadCount; j++) {
        double minTaskISPC = 1e30;
        for (int i = 0; i < 3; ++i) {
            double startTime = CycleTimer::currentSeconds();
            saxpy_ispc_withtasks(threads[j], N, scale, arrayX, arrayY, resultTasks);
            double endTime = CycleTimer::currentSeconds();
            minTaskISPC = std::min(minTaskISPC, endTime - startTime);
        }
        verifyResult(N, resultTasks, resultSerial);

        printf("[saxpy task %d ispc]:\t[%.3f] ms\t[%.3f] GB/s\t[%.3f] GFLOPS\n",
            threads[j],
            minTaskISPC * 1000,
            toBW(TOTAL_BYTES, minTaskISPC),
            toGFLOPS(TOTAL_FLOPS, minTaskISPC));

        printf("\t\t\t\t(%.2fx speedup from use of tasks)\n", minISPC/minTaskISPC);
    }

    for (int j = 0; j < threadCount; j++) {
        double minThreadAVX2 = 1e30;
        for (int i = 0; i < 3; ++i) {
            double startTime = CycleTimer::currentSeconds();
            saxpyAVX2Threads(threads[j], N, scale, arrayX, arrayY, resultAVX2Threads);
            double endTime = CycleTimer::currentSeconds();
            minThreadAVX2 = std::min(minThreadAVX2, endTime - startTime);
        }

        verifyResult(N, resultAVX2Threads, resultSerial);

        printf("[saxpy avx2 %d thr]:\t[%.3f] ms\t[%.3f] GB/s\t[%.3f] GFLOPS\n",
               threads[j],
               minThreadAVX2 * 1000,
               toBW(TOTAL_BYTES, minThreadAVX2),
               toGFLOPS(TOTAL_FLOPS, minThreadAVX2));

        printf("\t\t\t\t(%.2fx speedup from AVX2 threads)\n", minISPC/minThreadAVX2);
    }

    // verifyResult(N, resultTasks, resultSerial);

    // printf("[saxpy task ispc]:\t[%.3f] ms\t[%.3f] GB/s\t[%.3f] GFLOPS\n",
    //        minTaskISPC * 1000,
    //        toBW(TOTAL_BYTES, minTaskISPC),
    //        toGFLOPS(TOTAL_FLOPS, minTaskISPC));

    // printf("\t\t\t\t(%.2fx speedup from use of tasks)\n", minISPC/minTaskISPC);
    //printf("\t\t\t\t(%.2fx speedup from ISPC)\n", minSerial/minISPC);
    //printf("\t\t\t\t(%.2fx speedup from task ISPC)\n", minSerial/minTaskISPC);

    std::free(arrayX);
    std::free(arrayY);
    std::free(resultSerial);
    std::free(resultISPC);
    std::free(resultTasks);
    std::free(resultAVX2);
    std::free(resultAVX2Threads);

    return 0;
}
