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
UTAUTTS_WORLD_API int UtauTTSWorldAnalysisShape(
    UtauTTSWorldAnalysisShapeRequest* request, char* error, int error_capacity);
UTAUTTS_WORLD_API int UtauTTSWorldAnalyze(
    const UtauTTSWorldAnalysisRequest* request, char* error, int error_capacity);
UTAUTTS_WORLD_API int UtauTTSWorldSynthesize(
    const UtauTTSWorldSynthesisRequest* request, char* error, int error_capacity);
}
