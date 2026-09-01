#include <cuda_runtime.h>

#include <cmath>
#include <cfloat>
#include <cstdio>
#include <vector>

#ifdef _WIN32
#define UTAUTTS_EXPORT extern "C" __declspec(dllexport)
#else
#define UTAUTTS_EXPORT extern "C" __attribute__((visibility("default")))
#endif

namespace
{
    struct WorldFeatureUnit
    {
        int feature_offset;
        int feature_frames;
        double duration_ms;
        double position_ms;
        double length_ms;
        double fade_in_ms;
        double fade_out_ms;
        double skip_ms;
        double consonant_ms;
        double required_ms;
        double consonant_velocity;
        double volume;
    };

    thread_local char last_error[512] = {};
    constexpr float pi = 3.14159265358979323846f;

    void set_error(const char *operation, cudaError_t error)
    {
        std::snprintf(last_error, sizeof(last_error), "%s: %s", operation, cudaGetErrorString(error));
    }

    void copy_error(char *destination, int capacity)
    {
        if (destination && capacity > 0)
        {
            std::snprintf(destination, static_cast<size_t>(capacity), "%s", last_error);
        }
    }

    __device__ int clamp_int(int value, int low, int high)
    {
        return max(low, min(high, value));
    }

    __global__ void wsola_kernel(const float *source, int source_frames, float *accumulator, float *weights, float *result, int target_frames, int sample_rate)
    {
        __shared__ float scores[512];
        __shared__ int candidates[512];
        __shared__ int chosen;

        int window = min(min((40 * sample_rate + 500) / 1000, source_frames), target_frames);
        if (window & 1)
            --window;
        int hop = max(1, window / 2);
        int search = max(1, min((5 * sample_rate + 500) / 1000, window / 4));
        int max_start = max(0, source_frames - window);
        float ratio = static_cast<float>(source_frames) / static_cast<float>(target_frames);
        int previous_source = 0;

        for (int output = 0; output < target_frames; output += hop)
        {
            int expected = min(static_cast<int>(floorf(static_cast<float>(output) * ratio + 0.5f)), max_start);
            int low = max(0, expected - search);
            int high = min(max_start, expected + search);
            int count = high - low + 1;

            if (output == 0)
            {
                if (threadIdx.x == 0)
                    chosen = expected;
            }
            else
            {
                int reference = clamp_int(previous_source + hop, 0, max_start);
                float best_score = -FLT_MAX;
                int best_candidate = high + 1;
                for (int candidate_index = static_cast<int>(threadIdx.x); candidate_index < count; candidate_index += blockDim.x)
                {
                    int candidate = low + candidate_index;
                    int length = min(window / 2, min(source_frames - reference, source_frames - candidate));
                    if (length >= 4)
                    {
                        float numerator = 0.0f;
                        float left_energy = 0.0f;
                        float right_energy = 0.0f;
                        for (int index = 0; index < length; ++index)
                        {
                            float left = source[reference + index];
                            float right = source[candidate + index];
                            numerator += left * right;
                            left_energy += left * left;
                            right_energy += right * right;
                        }
                        float score = numerator / (sqrtf(left_energy * right_energy) + 1e-12f);
                        if (score > best_score || (score == best_score && candidate < best_candidate))
                        {
                            best_score = score;
                            best_candidate = candidate;
                        }
                    }
                }
                scores[threadIdx.x] = best_score;
                candidates[threadIdx.x] = best_candidate;
                __syncthreads();
                for (int stride = blockDim.x / 2; stride > 0; stride >>= 1)
                {
                    if (threadIdx.x < stride)
                    {
                        float right_score = scores[threadIdx.x + stride];
                        int right_candidate = candidates[threadIdx.x + stride];
                        if (right_score > scores[threadIdx.x] ||
                            (right_score == scores[threadIdx.x] && right_candidate < candidates[threadIdx.x]))
                        {
                            scores[threadIdx.x] = right_score;
                            candidates[threadIdx.x] = right_candidate;
                        }
                    }
                    __syncthreads();
                }
                if (threadIdx.x == 0)
                    chosen = candidates[0];
            }
            __syncthreads();
            for (int index = static_cast<int>(threadIdx.x); index < window && output + index < target_frames + window; index += blockDim.x)
            {
                float weight = 0.5f - 0.5f * cosf(2.0f * pi * static_cast<float>(index + 1) / static_cast<float>(window + 1));
                accumulator[output + index] += source[chosen + index] * weight;
                weights[output + index] += weight;
            }
            __syncthreads();
            previous_source = chosen;
        }
        for (int index = static_cast<int>(threadIdx.x); index < target_frames; index += blockDim.x)
        {
            result[index] = weights[index] > 1e-12f ? accumulator[index] / weights[index] : 0.0f;
        }
    }

    __device__ double clamp_double(double value, double low, double high)
    {
        return fmax(low, fmin(high, value));
    }

    __device__ double lerp_double(double left, double right, double fraction)
    {
        return left + (right - left) * clamp_double(fraction, 0.0, 1.0);
    }

    __global__ void world_feature_mix_kernel(const double *input_f0, int frames, int fft_size,
                                              const WorldFeatureUnit *units, int unit_count,
                                              const double *source_f0, const double *source_spectrum,
                                              const double *source_ap, double *output_f0,
                                              double *output_spectrum, double *output_ap)
    {
        const int frame = static_cast<int>(blockIdx.x);
        if (frame >= frames)
            return;
        const int bins = fft_size / 2 + 1;
        const double time_ms = static_cast<double>(frame) * 10.0;
        bool dirty = false;
        bool source_voiced = false;

        for (int bin = static_cast<int>(threadIdx.x); bin < bins; bin += static_cast<int>(blockDim.x))
        {
            output_spectrum[frame * bins + bin] = 1e-12;
            output_ap[frame * bins + bin] = 1.0;
        }

        for (int unit_index = 0; unit_index < unit_count; ++unit_index)
        {
            const WorldFeatureUnit unit = units[unit_index];
            const double local_ms = time_ms - unit.position_ms;
            if (local_ms < 0.0 || local_ms > unit.length_ms)
                continue;

            double weight = 1.0;
            if (unit.fade_in_ms > 0.0 && local_ms < unit.fade_in_ms)
                weight = local_ms / unit.fade_in_ms;
            const double remaining = unit.length_ms - local_ms;
            if (unit.fade_out_ms > 0.0 && remaining < unit.fade_out_ms)
                weight = fmin(weight, remaining / unit.fade_out_ms);
            if (weight <= 1e-6)
                continue;

            const double destination_ms = fmax(0.0, unit.skip_ms + local_ms);
            const double consonant_speed = pow(0.5, 1.0 - unit.consonant_velocity / 100.0);
            const double source_consonant = fmin(fmax(0.0, unit.consonant_ms), unit.duration_ms);
            const double destination_consonant = source_consonant / consonant_speed;
            double source_ms;
            if (destination_ms < destination_consonant)
            {
                source_ms = fmin(unit.duration_ms, destination_ms * consonant_speed);
            }
            else
            {
                const double destination_vowel = fmax(10.0, unit.required_ms - destination_consonant);
                const double source_vowel = fmax(0.0, unit.duration_ms - source_consonant);
                source_ms = fmin(unit.duration_ms, source_consonant +
                    (destination_ms - destination_consonant) * source_vowel / destination_vowel);
            }

            const double source_frame = source_ms / 10.0;
            const int left = clamp_int(static_cast<int>(floor(source_frame)), 0, unit.feature_frames - 1);
            const int right = min(left + 1, unit.feature_frames - 1);
            const double fraction = source_frame - static_cast<double>(left);
            if (threadIdx.x == 0)
            {
                const bool voiced = lerp_double(source_f0[unit.feature_offset + left],
                                                source_f0[unit.feature_offset + right], fraction) > 71.0;
                if (!dirty || weight > 0.5)
                    source_voiced = voiced;
            }
            const double volume_gain = fmax(0.0, unit.volume) / 100.0;
            for (int bin = static_cast<int>(threadIdx.x); bin < bins; bin += static_cast<int>(blockDim.x))
            {
                const int output_index = frame * bins + bin;
                const int left_index = (unit.feature_offset + left) * bins + bin;
                const int right_index = (unit.feature_offset + right) * bins + bin;
                const double spectrum = lerp_double(source_spectrum[left_index], source_spectrum[right_index], fraction) *
                                        volume_gain * volume_gain;
                const double aperiodicity = lerp_double(source_ap[left_index], source_ap[right_index], fraction);
                output_spectrum[output_index] += weight * spectrum;
                if (!dirty)
                    output_ap[output_index] = aperiodicity;
                else
                    output_ap[output_index] = output_ap[output_index] * (1.0 - weight) + aperiodicity * weight;
            }
            dirty = true;
        }

        if (threadIdx.x == 0)
            output_f0[frame] = dirty && source_voiced ? input_f0[frame] : 0.0;
    }

}

UTAUTTS_EXPORT int UtauTTSGPUAvailable(char *error_output, int error_capacity)
{
    int count = 0;
    cudaError_t error = cudaGetDeviceCount(&count);
    if (error != cudaSuccess)
    {
        set_error("cudaGetDeviceCount", error);
        copy_error(error_output, error_capacity);
        return 0;
    }
    if (count == 0)
    {
        std::snprintf(last_error, sizeof(last_error), "no CUDA device found");
        copy_error(error_output, error_capacity);
        return 0;
    }
    last_error[0] = '\0';
    copy_error(error_output, error_capacity);
    return 1;
}

UTAUTTS_EXPORT int UtauTTSGPUWSOLA(const double *source, int source_frames,
                                   double *result, int target_frames, int sample_rate,
                                   char *error_output, int error_capacity)
{
    if (!source || !result || source_frames < 16 || target_frames < 16 || sample_rate <= 0)
    {
        std::snprintf(last_error, sizeof(last_error), "invalid WSOLA input");
        copy_error(error_output, error_capacity);
        return 0;
    }
    cudaStream_t stream = nullptr;
    float *device_source = nullptr;
    float *device_accumulator = nullptr;
    float *device_weights = nullptr;
    float *device_result = nullptr;
    std::vector<float> host_source(source_frames);
    std::vector<float> host_result(target_frames);
    for (int index = 0; index < source_frames; ++index)
        host_source[index] = static_cast<float>(source[index]);
    const size_t source_bytes = static_cast<size_t>(source_frames) * sizeof(float);
    const size_t work_frames = static_cast<size_t>(target_frames) +
                               static_cast<size_t>(min(source_frames, (40 * sample_rate + 500) / 1000));
    const size_t work_bytes = work_frames * sizeof(float);
    const size_t result_bytes = static_cast<size_t>(target_frames) * sizeof(float);
    cudaError_t error = cudaStreamCreateWithFlags(&stream, cudaStreamNonBlocking);
    if (error == cudaSuccess)
        error = cudaMallocAsync(&device_source, source_bytes, stream);
    if (error == cudaSuccess)
        error = cudaMallocAsync(&device_accumulator, work_bytes, stream);
    if (error == cudaSuccess)
        error = cudaMallocAsync(&device_weights, work_bytes, stream);
    if (error == cudaSuccess)
        error = cudaMallocAsync(&device_result, result_bytes, stream);
    if (error == cudaSuccess)
        error = cudaMemcpyAsync(device_source, host_source.data(), source_bytes, cudaMemcpyHostToDevice, stream);
    if (error == cudaSuccess)
        error = cudaMemsetAsync(device_accumulator, 0, work_bytes, stream);
    if (error == cudaSuccess)
        error = cudaMemsetAsync(device_weights, 0, work_bytes, stream);
    if (error == cudaSuccess)
    {
        wsola_kernel<<<1, 512, 0, stream>>>(device_source, source_frames, device_accumulator,
                                            device_weights, device_result, target_frames, sample_rate);
        error = cudaGetLastError();
    }
    if (error == cudaSuccess)
        error = cudaMemcpyAsync(host_result.data(), device_result, result_bytes, cudaMemcpyDeviceToHost, stream);
    if (device_result)
        cudaFreeAsync(device_result, stream);
    if (device_weights)
        cudaFreeAsync(device_weights, stream);
    if (device_accumulator)
        cudaFreeAsync(device_accumulator, stream);
    if (device_source)
        cudaFreeAsync(device_source, stream);
    if (stream)
    {
        cudaError_t sync_error = cudaStreamSynchronize(stream);
        if (error == cudaSuccess)
            error = sync_error;
        cudaStreamDestroy(stream);
    }
    if (error != cudaSuccess)
    {
        set_error("CUDA WSOLA", error);
        copy_error(error_output, error_capacity);
        return 0;
    }
    for (int index = 0; index < target_frames; ++index)
        result[index] = static_cast<double>(host_result[index]);
    last_error[0] = '\0';
    copy_error(error_output, error_capacity);
    return 1;
}

UTAUTTS_EXPORT int UtauTTSGPUWorldFeatureMix(const double *input_f0, int frames, int fft_size,
                                             const WorldFeatureUnit *units, int unit_count,
                                             const double *source_f0, const double *source_spectrum,
                                             const double *source_ap, double *output_f0,
                                             double *output_spectrum, double *output_ap,
                                             char *error_output, int error_capacity)
{
    if (!input_f0 || !units || !source_f0 || !source_spectrum || !source_ap ||
        !output_f0 || !output_spectrum || !output_ap || frames < 2 || fft_size < 2 ||
        (fft_size & 1) != 0 || unit_count < 1)
    {
        std::snprintf(last_error, sizeof(last_error), "invalid WORLD feature mix input");
        copy_error(error_output, error_capacity);
        return 0;
    }

    int source_frames = 0;
    for (int index = 0; index < unit_count; ++index)
    {
        if (units[index].feature_offset < 0 || units[index].feature_frames < 2)
        {
            std::snprintf(last_error, sizeof(last_error), "invalid WORLD source feature range");
            copy_error(error_output, error_capacity);
            return 0;
        }
        source_frames = max(source_frames, units[index].feature_offset + units[index].feature_frames);
    }

    const int bins = fft_size / 2 + 1;
    const size_t input_f0_bytes = static_cast<size_t>(frames) * sizeof(double);
    const size_t unit_bytes = static_cast<size_t>(unit_count) * sizeof(WorldFeatureUnit);
    const size_t source_f0_bytes = static_cast<size_t>(source_frames) * sizeof(double);
    const size_t source_feature_bytes = static_cast<size_t>(source_frames) * static_cast<size_t>(bins) * sizeof(double);
    const size_t output_feature_bytes = static_cast<size_t>(frames) * static_cast<size_t>(bins) * sizeof(double);

    double *device_input_f0 = nullptr;
    WorldFeatureUnit *device_units = nullptr;
    double *device_source_f0 = nullptr;
    double *device_source_spectrum = nullptr;
    double *device_source_ap = nullptr;
    double *device_output_f0 = nullptr;
    double *device_output_spectrum = nullptr;
    double *device_output_ap = nullptr;
    cudaStream_t stream = nullptr;

    cudaError_t error = cudaStreamCreateWithFlags(&stream, cudaStreamNonBlocking);
    if (error == cudaSuccess) error = cudaMallocAsync(&device_input_f0, input_f0_bytes, stream);
    if (error == cudaSuccess) error = cudaMallocAsync(&device_units, unit_bytes, stream);
    if (error == cudaSuccess) error = cudaMallocAsync(&device_source_f0, source_f0_bytes, stream);
    if (error == cudaSuccess) error = cudaMallocAsync(&device_source_spectrum, source_feature_bytes, stream);
    if (error == cudaSuccess) error = cudaMallocAsync(&device_source_ap, source_feature_bytes, stream);
    if (error == cudaSuccess) error = cudaMallocAsync(&device_output_f0, input_f0_bytes, stream);
    if (error == cudaSuccess) error = cudaMallocAsync(&device_output_spectrum, output_feature_bytes, stream);
    if (error == cudaSuccess) error = cudaMallocAsync(&device_output_ap, output_feature_bytes, stream);
    if (error == cudaSuccess) error = cudaMemcpyAsync(device_input_f0, input_f0, input_f0_bytes, cudaMemcpyHostToDevice, stream);
    if (error == cudaSuccess) error = cudaMemcpyAsync(device_units, units, unit_bytes, cudaMemcpyHostToDevice, stream);
    if (error == cudaSuccess) error = cudaMemcpyAsync(device_source_f0, source_f0, source_f0_bytes, cudaMemcpyHostToDevice, stream);
    if (error == cudaSuccess) error = cudaMemcpyAsync(device_source_spectrum, source_spectrum, source_feature_bytes, cudaMemcpyHostToDevice, stream);
    if (error == cudaSuccess) error = cudaMemcpyAsync(device_source_ap, source_ap, source_feature_bytes, cudaMemcpyHostToDevice, stream);
    if (error == cudaSuccess)
    {
        world_feature_mix_kernel<<<frames, 256, 0, stream>>>(device_input_f0, frames, fft_size, device_units,
            unit_count, device_source_f0, device_source_spectrum, device_source_ap, device_output_f0,
            device_output_spectrum, device_output_ap);
        error = cudaGetLastError();
    }
    if (error == cudaSuccess) error = cudaMemcpyAsync(output_f0, device_output_f0, input_f0_bytes, cudaMemcpyDeviceToHost, stream);
    if (error == cudaSuccess) error = cudaMemcpyAsync(output_spectrum, device_output_spectrum, output_feature_bytes, cudaMemcpyDeviceToHost, stream);
    if (error == cudaSuccess) error = cudaMemcpyAsync(output_ap, device_output_ap, output_feature_bytes, cudaMemcpyDeviceToHost, stream);

    if (device_output_ap) cudaFreeAsync(device_output_ap, stream);
    if (device_output_spectrum) cudaFreeAsync(device_output_spectrum, stream);
    if (device_output_f0) cudaFreeAsync(device_output_f0, stream);
    if (device_source_ap) cudaFreeAsync(device_source_ap, stream);
    if (device_source_spectrum) cudaFreeAsync(device_source_spectrum, stream);
    if (device_source_f0) cudaFreeAsync(device_source_f0, stream);
    if (device_units) cudaFreeAsync(device_units, stream);
    if (device_input_f0) cudaFreeAsync(device_input_f0, stream);
    if (stream)
    {
        const cudaError_t sync_error = cudaStreamSynchronize(stream);
        if (error == cudaSuccess)
            error = sync_error;
        cudaStreamDestroy(stream);
    }
    if (error != cudaSuccess)
    {
        set_error("CUDA WORLD feature mix", error);
        copy_error(error_output, error_capacity);
        return 0;
    }

    last_error[0] = '\0';
    copy_error(error_output, error_capacity);
    return 1;
}
