#pragma once

#if defined(_WIN32)
#  if defined(UTAUTTS_WORLD_ENGINE_BUILD)
#    define UTAUTTS_WORLD_API __declspec(dllexport)
#  else
#    define UTAUTTS_WORLD_API __declspec(dllimport)
#  endif
#else
#  define UTAUTTS_WORLD_API __attribute__((visibility("default")))
#endif

struct UtauTTSWorldAnalysisShapeRequest {
  int sample_count;
  int sample_rate;
  double frame_period_ms;
  int frame_count;
  int fft_size;
};

struct UtauTTSWorldAnalysisRequest {
  const double* samples;
  int sample_count;
  int sample_rate;
  double frame_period_ms;
  const double* input_f0;
  int input_f0_count;
  double* f0;
  double* spectrum;
  double* aperiodicity;
};

struct UtauTTSWorldSynthesisRequest {
  const double* f0;
  int frame_count;
  const double* spectrum;
  const double* aperiodicity;
  int fft_size;
  double frame_period_ms;
  int sample_rate;
  double* output;
  int output_count;
};

extern "C" {
UTAUTTS_WORLD_API int UtauTTSWorldF0(const double* samples, int count, int rate,
    double frame_ms, double* output, int capacity, char* error, int error_capacity);
UTAUTTS_WORLD_API int UtauTTSWorldAnalysisShape(
    UtauTTSWorldAnalysisShapeRequest* request, char* error, int error_capacity);
UTAUTTS_WORLD_API int UtauTTSWorldAnalyze(
    const UtauTTSWorldAnalysisRequest* request, char* error, int error_capacity);
UTAUTTS_WORLD_API int UtauTTSWorldAnalyzeD4C(
    const UtauTTSWorldAnalysisRequest* request, double d4c_threshold,
    char* error, int error_capacity);
UTAUTTS_WORLD_API int UtauTTSWorldSynthesize(
    const UtauTTSWorldSynthesisRequest* request, char* error, int error_capacity);

// WebAssemblyなど構造体の受け渡しが面倒な環境向けのフラットな入口。
UTAUTTS_WORLD_API int UtauTTSWorldAnalyzeFlat(
    const double* samples, int sample_count, int sample_rate,
    double frame_period_ms, const double* input_f0, int input_f0_count,
    double* f0, double* spectrum, double* aperiodicity, char* error,
    int error_capacity);
// 逐次合成。パラメータを少しずつ足し、buffer_sizeサンプルずつ取り出す。区切りでも波形がつながる。
UTAUTTS_WORLD_API void* UtauTTSWorldStreamOpen(int sample_rate,
    double frame_period_ms, int fft_size, int buffer_size, int ring_size);
// 1なら足した。0ならたまっていて足せない（先に取り出す）。
UTAUTTS_WORLD_API int UtauTTSWorldStreamAdd(void* stream, const double* f0,
    int frame_count, const double* spectrum, const double* aperiodicity);
// 1ならoutputへbuffer_sizeサンプルを書いた。0なら足したパラメータがまだ足りない。
UTAUTTS_WORLD_API int UtauTTSWorldStreamSynthesize(void* stream, double* output);
UTAUTTS_WORLD_API void UtauTTSWorldStreamClose(void* stream);

UTAUTTS_WORLD_API int UtauTTSWorldSynthesizeFlat(
    const double* f0, int frame_count, const double* spectrum,
    const double* aperiodicity, int fft_size, double frame_period_ms,
    int sample_rate, double* output, int output_count, char* error,
    int error_capacity);
}
