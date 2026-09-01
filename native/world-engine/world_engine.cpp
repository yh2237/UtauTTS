#include "world_engine.h"

#include <algorithm>
#include <cstdio>
#include <exception>
#include <vector>

#include "world/cheaptrick.h"
#include "world/d4c.h"
#include "world/harvest.h"
#include "world/synthesis.h"

namespace {
constexpr double kF0Floor = 71.0;
constexpr double kF0Ceil = 1100.0;

int fail(const char* message, char* error, int capacity) {
  if (error != nullptr && capacity > 0) {
    std::snprintf(error, static_cast<size_t>(capacity), "%s", message);
  }
  return 0;
}

int synthesis_length(int frames, int sample_rate, double frame_period_ms) {
  if (frames < 2 || sample_rate <= 0 || frame_period_ms <= 0.0) return 0;
  return static_cast<int>((frames - 1) * frame_period_ms / 1000.0 *
                          sample_rate) + 1;
}
}

int UtauTTSWorldAnalysisShape(UtauTTSWorldAnalysisShapeRequest* request,
                              char* error, int error_capacity) {
  if (request == nullptr || request->sample_count < 2 ||
      request->sample_rate <= 0 || request->frame_period_ms <= 0.0) {
    return fail("invalid WORLD analysis shape", error, error_capacity);
  }
  HarvestOption harvest{};
  InitializeHarvestOption(&harvest);
  harvest.frame_period = request->frame_period_ms;
  harvest.f0_floor = kF0Floor;
  harvest.f0_ceil = kF0Ceil;
  CheapTrickOption cheaptrick{};
  InitializeCheapTrickOption(request->sample_rate, &cheaptrick);
  cheaptrick.f0_floor = kF0Floor;
  request->frame_count = GetSamplesForHarvest(
      request->sample_rate, request->sample_count, harvest.frame_period);
  request->fft_size =
      GetFFTSizeForCheapTrick(request->sample_rate, &cheaptrick);
  if (request->frame_count < 2 || request->fft_size <= 0) {
    return fail("WORLD returned an empty analysis shape", error,
                error_capacity);
  }
  return 1;
}

int UtauTTSWorldAnalyze(const UtauTTSWorldAnalysisRequest* request,
                        char* error, int error_capacity) {
  if (request == nullptr || request->samples == nullptr ||
      request->f0 == nullptr || request->spectrum == nullptr ||
      request->aperiodicity == nullptr) {
    return fail("invalid WORLD analysis buffers", error, error_capacity);
  }
  UtauTTSWorldAnalysisShapeRequest shape{
      request->sample_count, request->sample_rate, request->frame_period_ms, 0,
      0};
  if (!UtauTTSWorldAnalysisShape(&shape, error, error_capacity)) return 0;
  try {
    HarvestOption harvest{};
    InitializeHarvestOption(&harvest);
    harvest.frame_period = request->frame_period_ms;
    harvest.f0_floor = kF0Floor;
    harvest.f0_ceil = kF0Ceil;
    const int frame_count = request->input_f0 != nullptr &&
                                    request->input_f0_count >= 2
                                ? request->input_f0_count
                                : shape.frame_count;
    std::vector<double> time_axis(static_cast<size_t>(frame_count));
    for (int frame = 0; frame < frame_count; ++frame) {
      time_axis[frame] = frame * request->frame_period_ms / 1000.0;
    }
    if (request->input_f0 != nullptr && request->input_f0_count == frame_count) {
      std::copy(request->input_f0, request->input_f0 + frame_count,
                request->f0);
    } else {
      Harvest(request->samples, request->sample_count, request->sample_rate,
              &harvest, time_axis.data(), request->f0);
    }

    const int bins = shape.fft_size / 2 + 1;
    std::vector<double*> spectrum_rows(static_cast<size_t>(frame_count));
    std::vector<double*> aperiodicity_rows(
        static_cast<size_t>(frame_count));
    for (int frame = 0; frame < frame_count; ++frame) {
      spectrum_rows[frame] =
          request->spectrum + static_cast<size_t>(frame) * bins;
      aperiodicity_rows[frame] =
          request->aperiodicity + static_cast<size_t>(frame) * bins;
    }

    CheapTrickOption cheaptrick{};
    InitializeCheapTrickOption(request->sample_rate, &cheaptrick);
    cheaptrick.f0_floor = kF0Floor;
    cheaptrick.fft_size = shape.fft_size;
    CheapTrick(request->samples, request->sample_count, request->sample_rate,
               time_axis.data(), request->f0, frame_count, &cheaptrick,
               spectrum_rows.data());
    D4COption d4c{};
    InitializeD4COption(&d4c);
    d4c.threshold = 0;
    D4C(request->samples, request->sample_count, request->sample_rate,
        time_axis.data(), request->f0, frame_count, shape.fft_size, &d4c,
        aperiodicity_rows.data());
    return 1;
  } catch (const std::exception& exception) {
    return fail(exception.what(), error, error_capacity);
  } catch (...) {
    return fail("unknown WORLD analysis error", error, error_capacity);
  }
}

int UtauTTSWorldSynthesize(const UtauTTSWorldSynthesisRequest* request,
                           char* error, int error_capacity) {
  if (request == nullptr) {
    return fail("invalid WORLD synthesis request", error, error_capacity);
  }
  const int required = synthesis_length(request->frame_count,
                                        request->sample_rate,
                                        request->frame_period_ms);
  if (request->f0 == nullptr || request->spectrum == nullptr ||
      request->aperiodicity == nullptr || request->output == nullptr ||
      request->fft_size <= 0 || request->output_count < required ||
      required <= 0) {
    return fail("invalid WORLD synthesis buffers", error, error_capacity);
  }
  try {
    const int bins = request->fft_size / 2 + 1;
    std::vector<double*> spectrum_rows(
        static_cast<size_t>(request->frame_count));
    std::vector<double*> aperiodicity_rows(
        static_cast<size_t>(request->frame_count));
    for (int frame = 0; frame < request->frame_count; ++frame) {
      spectrum_rows[frame] = const_cast<double*>(
          request->spectrum + static_cast<size_t>(frame) * bins);
      aperiodicity_rows[frame] = const_cast<double*>(
          request->aperiodicity + static_cast<size_t>(frame) * bins);
    }
    std::fill(request->output, request->output + request->output_count, 0.0);
    Synthesis(request->f0, request->frame_count, spectrum_rows.data(),
              aperiodicity_rows.data(), request->fft_size,
              request->frame_period_ms, request->sample_rate, required,
              request->output);
    return 1;
  } catch (const std::exception& exception) {
    return fail(exception.what(), error, error_capacity);
  } catch (...) {
    return fail("unknown WORLD synthesis error", error, error_capacity);
  }
}
