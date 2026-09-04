using System.Text.Json;
using System.Text.Json.Serialization;
using Microsoft.ML.OnnxRuntime;
using Microsoft.ML.OnnxRuntime.Tensors;

if (args.Length == 2 && args[0] == "--inspect") {
    try {
        using var session = new InferenceSession(args[1]);
        var metadata = new {
            inputs = session.InputMetadata.ToDictionary(
                item => item.Key,
                item => new { type = item.Value.ElementType.Name, dimensions = item.Value.Dimensions }),
            outputs = session.OutputMetadata.ToDictionary(
                item => item.Key,
                item => new { type = item.Value.ElementType.Name, dimensions = item.Value.Dimensions }),
        };
        Console.WriteLine(JsonSerializer.Serialize(metadata));
        return 0;
    } catch (Exception error) {
        Console.Error.WriteLine(error.Message);
        return 1;
    }
}

if (args.Length != 1) {
    Console.Error.WriteLine("usage: utautts-diffsinger-bridge REQUEST.json | --inspect MODEL.onnx");
    return 2;
}

try {
    var json = File.ReadAllText(args[0]);
    var request = JsonSerializer.Deserialize<Request>(json)
        ?? throw new InvalidDataException("request is empty");
    Validate(request);
    var samples = Render(request);
    WriteWave(request.OutputPath, request.SampleRate, samples);
    return 0;
} catch (Exception error) {
    Console.Error.WriteLine(error.Message);
    return 1;
}

static float[] Render(Request request) {
    var durations = request.DurationLinguisticPath.Length > 0
        ? BlendDurations(request, PredictDurations(request))
        : request.Durations;
    var f0 = request.PitchLinguisticPath.Length > 0
        ? BlendPitch(request.F0, PredictPitch(request, durations), request.PitchPredictorMix)
        : request.F0;
    var variance = request.VarianceLinguisticPath.Length > 0
        ? PredictVariance(request, durations, f0)
        : new VarianceResult();
    using var acoustic = new InferenceSession(request.AcousticPath);
    var acousticInputs = new List<NamedOnnxValue>();
    acousticInputs.Add(Tensor("tokens", request.Tokens, 1, request.Tokens.Length));
    acousticInputs.Add(Tensor("durations", durations, 1, durations.Length));
    acousticInputs.Add(Tensor("f0", f0, 1, f0.Length));
    if (request.Languages is { Length: > 0 }) {
        acousticInputs.Add(Tensor("languages", request.Languages, 1, request.Languages.Length));
    }
    if (request.UseGender) {
        acousticInputs.Add(Tensor("gender", Enumerable.Repeat(0f, request.F0.Length).ToArray(), 1, request.F0.Length));
    }
    if (request.UseVelocity) {
        acousticInputs.Add(Tensor("velocity", Enumerable.Repeat(1f, request.F0.Length).ToArray(), 1, request.F0.Length));
    }
    if (request.UseEnergy) {
        acousticInputs.Add(Tensor("energy", RequiredVariance(variance.Energy, "energy"), 1, request.F0.Length));
    }
    if (request.UseBreathiness) {
        acousticInputs.Add(Tensor("breathiness", RequiredVariance(variance.Breathiness, "breathiness"), 1, request.F0.Length));
    }
    if (request.UseVoicing) {
        acousticInputs.Add(Tensor("voicing", RequiredVariance(variance.Voicing, "voicing"), 1, request.F0.Length));
    }
    if (request.UseTension) {
        acousticInputs.Add(Tensor("tension", RequiredVariance(variance.Tension, "tension"), 1, request.F0.Length));
    }
    if (request.SpeakerEmbed is { Length: > 0 }) {
        var values = new float[request.F0.Length * request.SpeakerEmbed.Length];
        for (var frame = 0; frame < request.F0.Length; frame++) {
            request.SpeakerEmbed.CopyTo(values, frame * request.SpeakerEmbed.Length);
        }
        acousticInputs.Add(Tensor("spk_embed", values, 1, request.F0.Length, request.SpeakerEmbed.Length));
    }
    if (request.UseContinuousAcceleration) {
        if (request.UseVariableDepth) {
            acousticInputs.Add(Tensor("depth", new[] { request.Depth }, 1));
        }
        acousticInputs.Add(Tensor("steps", new[] { request.Steps }, 1));
    } else {
        if (request.UseVariableDepth) {
            var discreteDepth = (long)Math.Round(request.Depth * 1000);
            var speedup = Math.Max(1, discreteDepth / request.Steps);
            discreteDepth = discreteDepth / speedup * speedup;
            acousticInputs.Add(Tensor("depth", new[] { discreteDepth }, 1));
            acousticInputs.Add(Tensor("speedup", new[] { speedup }, 1));
        } else {
            acousticInputs.Add(Tensor("speedup", new[] { request.Speedup }, 1));
        }
    }
    CheckInputs(acoustic, acousticInputs, "acoustic");
    using var acousticOutputs = acoustic.Run(acousticInputs);
    var acousticMel = acousticOutputs.First().AsTensor<float>();
    var melValues = acousticMel.ToArray();
    if (request.MelScale != 0 && request.MelScale != 1) {
        for (var index = 0; index < melValues.Length; index++) melValues[index] *= request.MelScale;
    }
    var mel = new DenseTensor<float>(melValues, acousticMel.Dimensions.ToArray());

    using var vocoder = new InferenceSession(request.VocoderPath);
    var vocoderInputs = new List<NamedOnnxValue>();
    vocoderInputs.Add(NamedOnnxValue.CreateFromTensor("mel", mel));
    vocoderInputs.Add(Tensor("f0", f0, 1, f0.Length));
    CheckInputs(vocoder, vocoderInputs, "vocoder");
    using var vocoderOutputs = vocoder.Run(vocoderInputs);
    return vocoderOutputs.First().AsTensor<float>().ToArray();
}

static float[] RequiredVariance(float[]? values, string name) {
    if (values is null) {
        throw new InvalidDataException($"acoustic model requires {name}, but the variance model does not predict it");
    }
    var minimum = name == "tension" ? -10f : -96f;
    var maximum = name == "tension" ? 10f : 0f;
    return values.Select(value => Math.Clamp(value, minimum, maximum)).ToArray();
}

static VarianceResult PredictVariance(Request request, long[] durations, float[] f0) {
    using var linguistic = new InferenceSession(request.VarianceLinguisticPath);
    var linguisticInputs = new List<NamedOnnxValue> {
        Tensor("tokens", request.VarianceTokens!, 1, request.VarianceTokens!.Length),
    };
    if (request.VariancePredictsDur) {
        linguisticInputs.Add(Tensor("word_div", request.WordDiv!, 1, request.WordDiv!.Length));
        linguisticInputs.Add(Tensor("word_dur", request.WordDur!, 1, request.WordDur!.Length));
    } else {
        linguisticInputs.Add(Tensor("ph_dur", durations, 1, durations.Length));
    }
    if (request.VarianceLanguages is { Length: > 0 }) {
        linguisticInputs.Add(Tensor("languages", request.VarianceLanguages, 1, request.VarianceLanguages.Length));
    }
    CheckInputs(linguistic, linguisticInputs, "variance linguistic");
    using var linguisticOutputs = linguistic.Run(linguisticInputs);
    var encoder = linguisticOutputs.First(value => value.Name == "encoder_out").AsTensor<float>();

    var totalFrames = f0.Length;
    var midi = f0.Select(value => value > 0
        ? 69f + 12f * (float)Math.Log2(value / 440f)
        : 0f).ToArray();
    var predictorInputs = new List<NamedOnnxValue> {
        NamedOnnxValue.CreateFromTensor("encoder_out", encoder),
        Tensor("ph_dur", durations, 1, durations.Length),
        Tensor("pitch", midi, 1, totalFrames),
    };
    var predictedCount = 0;
    void AddSeed(string name, bool enabled) {
        if (!enabled) return;
        predictorInputs.Add(Tensor(name, new float[totalFrames], 1, totalFrames));
        predictedCount++;
    }
    AddSeed("energy", request.VariancePredictsEnergy);
    AddSeed("breathiness", request.VariancePredictsBreath);
    AddSeed("voicing", request.VariancePredictsVoicing);
    AddSeed("tension", request.VariancePredictsTension);
    predictorInputs.Add(Tensor("retake", Enumerable.Repeat(true, totalFrames * predictedCount).ToArray(), 1, totalFrames, predictedCount));
    if (request.VarianceContinuous) {
        predictorInputs.Add(Tensor("steps", new[] { request.Steps }, 1));
    } else {
        predictorInputs.Add(Tensor("speedup", new[] { request.Speedup }, 1));
    }
    if (request.VarianceSpeakerEmbed is { Length: > 0 }) {
        var values = new float[totalFrames * request.VarianceSpeakerEmbed.Length];
        for (var frame = 0; frame < totalFrames; frame++) {
            request.VarianceSpeakerEmbed.CopyTo(values, frame * request.VarianceSpeakerEmbed.Length);
        }
        predictorInputs.Add(Tensor("spk_embed", values, 1, totalFrames, request.VarianceSpeakerEmbed.Length));
    }
    using var predictor = new InferenceSession(request.VariancePredictorPath);
    CheckInputs(predictor, predictorInputs, "variance predictor");
    using var outputs = predictor.Run(predictorInputs);
    float[]? Output(string name, bool enabled) {
        if (!enabled) return null;
        var values = outputs.First(value => value.Name == name).AsTensor<float>().ToArray();
        if (values.Length != totalFrames) throw new InvalidDataException($"{name} returned an invalid frame count");
        return values;
    }
    return new VarianceResult {
        Energy = Output("energy_pred", request.VariancePredictsEnergy),
        Breathiness = Output("breathiness_pred", request.VariancePredictsBreath),
        Voicing = Output("voicing_pred", request.VariancePredictsVoicing),
        Tension = Output("tension_pred", request.VariancePredictsTension),
    };
}

static long[] BlendDurations(Request request, long[] predicted) {
    if (request.DurationPredictorMix <= 0) return request.Durations;
    var weights = new float[predicted.Length];
    for (var index = 0; index < weights.Length; index++) {
        weights[index] = request.Durations[index] * (1 - request.DurationPredictorMix)
            + predicted[index] * request.DurationPredictorMix;
    }
    return FitDurations(weights, request.WordDiv!, request.WordDur!);
}

static float[] BlendPitch(float[] requested, float[] predicted, float mix) {
    if (mix <= 0) return requested;
    var result = new float[requested.Length];
    for (var index = 0; index < result.Length; index++) {
        var left = requested[index];
        var right = predicted[index];
        result[index] = left > 0 && right > 0 && float.IsFinite(right)
            ? (float)Math.Exp(Math.Log(left) * (1 - mix) + Math.Log(right) * mix)
            : left;
    }
    return result;
}

static float[] PredictPitch(Request request, long[] durations) {
    using var linguistic = new InferenceSession(request.PitchLinguisticPath);
    var linguisticInputs = new List<NamedOnnxValue> {
        Tensor("tokens", request.PitchTokens!, 1, request.PitchTokens!.Length),
    };
    if (request.PitchPredictsDur) {
        linguisticInputs.Add(Tensor("word_div", request.WordDiv!, 1, request.WordDiv!.Length));
        linguisticInputs.Add(Tensor("word_dur", request.WordDur!, 1, request.WordDur!.Length));
    } else {
        linguisticInputs.Add(Tensor("ph_dur", durations, 1, durations.Length));
    }
    if (request.PitchLanguages is { Length: > 0 }) {
        linguisticInputs.Add(Tensor("languages", request.PitchLanguages, 1, request.PitchLanguages.Length));
    }
    CheckInputs(linguistic, linguisticInputs, "pitch linguistic");
    using var linguisticOutputs = linguistic.Run(linguisticInputs);
    var encoder = linguisticOutputs.First(value => value.Name == "encoder_out").AsTensor<float>();

    using var predictor = new InferenceSession(request.PitchPredictorPath);
    var totalFrames = request.F0.Length;
    var predictorInputs = new List<NamedOnnxValue> {
        NamedOnnxValue.CreateFromTensor("encoder_out", encoder),
        Tensor("ph_dur", durations, 1, durations.Length),
        Tensor("note_midi", request.NoteMIDI!, 1, request.NoteMIDI!.Length),
        Tensor("note_dur", request.WordDur!, 1, request.WordDur!.Length),
        Tensor("pitch", Enumerable.Repeat(60f, totalFrames).ToArray(), 1, totalFrames),
        Tensor("retake", Enumerable.Repeat(true, totalFrames).ToArray(), 1, totalFrames),
    };
    if (request.PitchContinuous) {
        predictorInputs.Add(Tensor("steps", new[] { request.Steps }, 1));
    } else {
        predictorInputs.Add(Tensor("speedup", new[] { request.Speedup }, 1));
    }
    if (request.PitchUseExpr) {
        predictorInputs.Add(Tensor("expr", Enumerable.Repeat(1f, totalFrames).ToArray(), 1, totalFrames));
    }
    if (request.PitchUseNoteRest) {
        predictorInputs.Add(Tensor("note_rest", request.NoteRest!, 1, request.NoteRest!.Length));
    }
    if (request.PitchSpeakerEmbed is { Length: > 0 }) {
        var values = new float[totalFrames * request.PitchSpeakerEmbed.Length];
        for (var frame = 0; frame < totalFrames; frame++) {
            request.PitchSpeakerEmbed.CopyTo(values, frame * request.PitchSpeakerEmbed.Length);
        }
        predictorInputs.Add(Tensor("spk_embed", values, 1, totalFrames, request.PitchSpeakerEmbed.Length));
    }
    CheckInputs(predictor, predictorInputs, "pitch predictor");
    using var predictorOutputs = predictor.Run(predictorInputs);
    var midi = predictorOutputs.First().AsTensor<float>().ToArray();
    if (midi.Length != totalFrames) throw new InvalidDataException("pitch predictor returned an invalid frame count");
    var result = new float[totalFrames];
    var frameOffset = 0;
    for (var note = 0; note < request.WordDur!.Length; note++) {
        var frames = checked((int)request.WordDur[note]);
        var baseHz = 440 * Math.Pow(2, (request.NoteMIDI![note] - 69) / 12);
        for (var frame = 0; frame < frames; frame++) {
            var index = frameOffset + frame;
            var predictedHz = 440 * Math.Pow(2, (midi[index] - 69) / 12);
            result[index] = (float)(predictedHz * request.F0[index] / baseHz);
        }
        frameOffset += frames;
    }
    return result;
}

static long[] PredictDurations(Request request) {
    using var linguistic = new InferenceSession(request.DurationLinguisticPath);
    var linguisticInputs = new List<NamedOnnxValue> {
        Tensor("tokens", request.DurationTokens!, 1, request.DurationTokens!.Length),
        Tensor("word_div", request.WordDiv!, 1, request.WordDiv!.Length),
        Tensor("word_dur", request.WordDur!, 1, request.WordDur!.Length),
    };
    if (request.DurationLanguages is { Length: > 0 }) {
        linguisticInputs.Add(Tensor("languages", request.DurationLanguages, 1, request.DurationLanguages.Length));
    }
    CheckInputs(linguistic, linguisticInputs, "duration linguistic");
    using var linguisticOutputs = linguistic.Run(linguisticInputs);
    var encoder = linguisticOutputs.First(value => value.Name == "encoder_out").AsTensor<float>();
    var masks = linguisticOutputs.First(value => value.Name == "x_masks").AsTensor<bool>();

    using var predictor = new InferenceSession(request.DurationPredictorPath);
    var predictorInputs = new List<NamedOnnxValue> {
        NamedOnnxValue.CreateFromTensor("encoder_out", encoder),
        NamedOnnxValue.CreateFromTensor("x_masks", masks),
        Tensor("ph_midi", request.PhMIDI!, 1, request.PhMIDI!.Length),
    };
    if (request.DurationSpeakerEmbed is { Length: > 0 }) {
        var values = new float[request.DurationTokens!.Length * request.DurationSpeakerEmbed.Length];
        for (var phone = 0; phone < request.DurationTokens.Length; phone++) {
            request.DurationSpeakerEmbed.CopyTo(values, phone * request.DurationSpeakerEmbed.Length);
        }
        predictorInputs.Add(Tensor("spk_embed", values, 1, request.DurationTokens.Length, request.DurationSpeakerEmbed.Length));
    }
    CheckInputs(predictor, predictorInputs, "duration predictor");
    using var predictorOutputs = predictor.Run(predictorInputs);
    return FitDurations(predictorOutputs.First().AsTensor<float>().ToArray(), request.WordDiv!, request.WordDur!);
}

static long[] FitDurations(float[] predicted, long[] wordDiv, long[] wordDur) {
    if (predicted.Length != wordDiv.Sum()) throw new InvalidDataException("duration predictor returned an invalid phoneme count");
    var result = new long[predicted.Length];
    var phoneOffset = 0;
    for (var word = 0; word < wordDiv.Length; word++) {
        var count = checked((int)wordDiv[word]);
        var target = wordDur[word];
        if (count == 1) {
            result[phoneOffset++] = target;
            continue;
        }
        var remaining = Math.Max(0, target - count);
        var weights = new double[count];
        var sum = 0d;
        for (var index = 0; index < count; index++) {
            var value = predicted[phoneOffset + index];
            weights[index] = float.IsFinite(value) ? Math.Max(0, value) : 0;
            sum += weights[index];
        }
        if (sum <= 0) {
            Array.Fill(weights, 1d);
            sum = count;
        }
        var used = 0L;
        var fractions = new (int Index, double Fraction)[count];
        for (var index = 0; index < count; index++) {
            var exact = remaining * weights[index] / sum;
            var whole = (long)Math.Floor(exact);
            result[phoneOffset + index] = 1 + whole;
            used += whole;
            fractions[index] = (index, exact - whole);
        }
        foreach (var item in fractions.OrderByDescending(item => item.Fraction).Take(checked((int)(remaining - used)))) {
            result[phoneOffset + item.Index]++;
        }
        phoneOffset += count;
    }
    return result;
}

static NamedOnnxValue Tensor<T>(string name, T[] data, params int[] dimensions) {
    return NamedOnnxValue.CreateFromTensor(name, new DenseTensor<T>(data, dimensions));
}

static void CheckInputs(InferenceSession session, IEnumerable<NamedOnnxValue> inputs, string model) {
    var supplied = inputs.Select(value => value.Name).ToHashSet(StringComparer.Ordinal);
    var missing = session.InputMetadata.Keys.Where(name => !supplied.Contains(name)).ToArray();
    if (missing.Length > 0) {
        throw new InvalidDataException($"{model} model requires unsupported inputs: {string.Join(", ", missing)}");
    }
}

static void Validate(Request request) {
    if (request.Version != 1) throw new InvalidDataException("unsupported request version");
    if (!File.Exists(request.AcousticPath)) throw new FileNotFoundException("acoustic model not found", request.AcousticPath);
    if (!File.Exists(request.VocoderPath)) throw new FileNotFoundException("vocoder model not found", request.VocoderPath);
    if (request.Tokens.Length == 0 || request.Tokens.Length != request.Durations.Length) {
        throw new InvalidDataException("tokens and durations must have the same positive length");
    }
    if (request.F0.Length == 0 || request.Durations.Sum() != request.F0.Length) {
        throw new InvalidDataException("duration sum must match f0 length");
    }
    if (request.Steps <= 0) throw new InvalidDataException("steps must be positive");
    if (request.SampleRate <= 0) throw new InvalidDataException("sample rate must be positive");
    if (request.DurationLinguisticPath.Length > 0) {
        if (!File.Exists(request.DurationLinguisticPath)) throw new FileNotFoundException("duration linguistic model not found", request.DurationLinguisticPath);
        if (!File.Exists(request.DurationPredictorPath)) throw new FileNotFoundException("duration predictor model not found", request.DurationPredictorPath);
        if (request.DurationTokens is not { Length: > 0 } || request.DurationTokens.Length != request.Tokens.Length) {
            throw new InvalidDataException("duration tokens must match acoustic tokens");
        }
        if (request.WordDiv is not { Length: > 0 } || request.WordDur is null || request.WordDiv.Length != request.WordDur.Length || request.WordDiv.Sum() != request.DurationTokens.Length) {
            throw new InvalidDataException("word divisions and durations are invalid");
        }
        if (request.WordDur.Any(value => value <= 0) || request.WordDiv.Any(value => value <= 0)) {
            throw new InvalidDataException("word divisions and durations must be positive");
        }
        if (request.WordDiv.Zip(request.WordDur).Any(pair => pair.First > pair.Second)) {
            throw new InvalidDataException("a word must have at least one frame per phoneme");
        }
        if (request.PhMIDI is null || request.PhMIDI.Length != request.DurationTokens.Length) {
            throw new InvalidDataException("ph_midi must match duration tokens");
        }
        if (request.DurationLanguages is { Length: > 0 } && request.DurationLanguages.Length != request.DurationTokens.Length) {
            throw new InvalidDataException("duration languages must match duration tokens");
        }
        if (request.DurationPredictorMix is < 0 or > 1) {
            throw new InvalidDataException("duration predictor mix must be between 0 and 1");
        }
    }
    if (request.PitchLinguisticPath.Length > 0) {
        if (!File.Exists(request.PitchLinguisticPath)) throw new FileNotFoundException("pitch linguistic model not found", request.PitchLinguisticPath);
        if (!File.Exists(request.PitchPredictorPath)) throw new FileNotFoundException("pitch predictor model not found", request.PitchPredictorPath);
        if (request.PitchTokens is not { Length: > 0 } || request.PitchTokens.Length != request.Tokens.Length) {
            throw new InvalidDataException("pitch tokens must match acoustic tokens");
        }
        if (request.WordDiv is not { Length: > 0 } || request.WordDur is null || request.WordDiv.Length != request.WordDur.Length || request.WordDiv.Sum() != request.PitchTokens.Length) {
            throw new InvalidDataException("pitch word divisions and durations are invalid");
        }
        if (request.NoteMIDI is null || request.NoteRest is null || request.NoteMIDI.Length != request.WordDur.Length || request.NoteRest.Length != request.WordDur.Length) {
            throw new InvalidDataException("pitch note arrays must match word durations");
        }
        if (request.WordDiv.Any(value => value <= 0) || request.WordDur.Any(value => value <= 0) || request.WordDur.Sum() != request.F0.Length) {
            throw new InvalidDataException("pitch word divisions and durations must be positive and match frame count");
        }
        if (request.PitchLanguages is { Length: > 0 } && request.PitchLanguages.Length != request.PitchTokens.Length) {
            throw new InvalidDataException("pitch languages must match pitch tokens");
        }
        if (request.PitchPredictorMix is < 0 or > 1) {
            throw new InvalidDataException("pitch predictor mix must be between 0 and 1");
        }
    }
    if (request.VarianceLinguisticPath.Length > 0) {
        if (!File.Exists(request.VarianceLinguisticPath)) throw new FileNotFoundException("variance linguistic model not found", request.VarianceLinguisticPath);
        if (!File.Exists(request.VariancePredictorPath)) throw new FileNotFoundException("variance predictor model not found", request.VariancePredictorPath);
        if (request.VarianceTokens is not { Length: > 0 } || request.VarianceTokens.Length != request.Tokens.Length) {
            throw new InvalidDataException("variance tokens must match acoustic tokens");
        }
        if (request.VariancePredictsDur && (request.WordDiv is not { Length: > 0 } || request.WordDur is null || request.WordDiv.Length != request.WordDur.Length || request.WordDiv.Sum() != request.VarianceTokens.Length)) {
            throw new InvalidDataException("variance word divisions and durations are invalid");
        }
        if (request.VarianceLanguages is { Length: > 0 } && request.VarianceLanguages.Length != request.VarianceTokens.Length) {
            throw new InvalidDataException("variance languages must match variance tokens");
        }
        if (!(request.VariancePredictsEnergy || request.VariancePredictsBreath || request.VariancePredictsVoicing || request.VariancePredictsTension)) {
            throw new InvalidDataException("variance model does not predict any variance parameters");
        }
    } else if (request.UseEnergy || request.UseBreathiness || request.UseVoicing || request.UseTension) {
        throw new InvalidDataException("acoustic model requires a variance model");
    }
}

static void WriteWave(string path, int sampleRate, float[] samples) {
    if (samples.Length == 0) throw new InvalidDataException("vocoder returned no samples");
    Directory.CreateDirectory(Path.GetDirectoryName(Path.GetFullPath(path))!);
    using var stream = File.Create(path);
    using var writer = new BinaryWriter(stream);
    var dataSize = checked(samples.Length * sizeof(short));
    writer.Write("RIFF"u8);
    writer.Write(36 + dataSize);
    writer.Write("WAVE"u8);
    writer.Write("fmt "u8);
    writer.Write(16);
    writer.Write((short)1);
    writer.Write((short)1);
    writer.Write(sampleRate);
    writer.Write(sampleRate * sizeof(short));
    writer.Write((short)sizeof(short));
    writer.Write((short)16);
    writer.Write("data"u8);
    writer.Write(dataSize);
    foreach (var sample in samples) {
        writer.Write((short)Math.Round(Math.Clamp(sample, -1f, 1f) * short.MaxValue));
    }
}

sealed class Request {
    [JsonPropertyName("version")] public int Version { get; set; }
    [JsonPropertyName("acoustic_path")] public string AcousticPath { get; set; } = "";
    [JsonPropertyName("vocoder_path")] public string VocoderPath { get; set; } = "";
    [JsonPropertyName("output_path")] public string OutputPath { get; set; } = "";
    [JsonPropertyName("tokens")] public long[] Tokens { get; set; } = [];
    [JsonPropertyName("durations")] public long[] Durations { get; set; } = [];
    [JsonPropertyName("f0")] public float[] F0 { get; set; } = [];
    [JsonPropertyName("sample_rate")] public int SampleRate { get; set; }
    [JsonPropertyName("steps")] public long Steps { get; set; }
    [JsonPropertyName("speedup")] public long Speedup { get; set; }
    [JsonPropertyName("depth")] public float Depth { get; set; }
    [JsonPropertyName("use_continuous_acceleration")] public bool UseContinuousAcceleration { get; set; }
    [JsonPropertyName("use_variable_depth")] public bool UseVariableDepth { get; set; }
    [JsonPropertyName("pitch_controllable")] public bool PitchControllable { get; set; }
    [JsonPropertyName("languages")] public long[]? Languages { get; set; }
    [JsonPropertyName("use_gender")] public bool UseGender { get; set; }
    [JsonPropertyName("use_velocity")] public bool UseVelocity { get; set; }
    [JsonPropertyName("use_energy")] public bool UseEnergy { get; set; }
    [JsonPropertyName("use_breathiness")] public bool UseBreathiness { get; set; }
    [JsonPropertyName("use_voicing")] public bool UseVoicing { get; set; }
    [JsonPropertyName("use_tension")] public bool UseTension { get; set; }
    [JsonPropertyName("speaker_embed")] public float[]? SpeakerEmbed { get; set; }
    [JsonPropertyName("duration_linguistic_path")] public string DurationLinguisticPath { get; set; } = "";
    [JsonPropertyName("duration_predictor_path")] public string DurationPredictorPath { get; set; } = "";
    [JsonPropertyName("duration_tokens")] public long[]? DurationTokens { get; set; }
    [JsonPropertyName("duration_languages")] public long[]? DurationLanguages { get; set; }
    [JsonPropertyName("duration_speaker_embed")] public float[]? DurationSpeakerEmbed { get; set; }
    [JsonPropertyName("duration_predictor_mix")] public float DurationPredictorMix { get; set; }
    [JsonPropertyName("word_div")] public long[]? WordDiv { get; set; }
    [JsonPropertyName("word_dur")] public long[]? WordDur { get; set; }
    [JsonPropertyName("ph_midi")] public long[]? PhMIDI { get; set; }
    [JsonPropertyName("pitch_linguistic_path")] public string PitchLinguisticPath { get; set; } = "";
    [JsonPropertyName("pitch_predictor_path")] public string PitchPredictorPath { get; set; } = "";
    [JsonPropertyName("pitch_tokens")] public long[]? PitchTokens { get; set; }
    [JsonPropertyName("pitch_languages")] public long[]? PitchLanguages { get; set; }
    [JsonPropertyName("pitch_speaker_embed")] public float[]? PitchSpeakerEmbed { get; set; }
    [JsonPropertyName("pitch_predicts_dur")] public bool PitchPredictsDur { get; set; }
    [JsonPropertyName("pitch_continuous")] public bool PitchContinuous { get; set; }
    [JsonPropertyName("pitch_use_expr")] public bool PitchUseExpr { get; set; }
    [JsonPropertyName("pitch_use_note_rest")] public bool PitchUseNoteRest { get; set; }
    [JsonPropertyName("pitch_predictor_mix")] public float PitchPredictorMix { get; set; }
    [JsonPropertyName("note_midi")] public float[]? NoteMIDI { get; set; }
    [JsonPropertyName("note_rest")] public bool[]? NoteRest { get; set; }
    [JsonPropertyName("variance_linguistic_path")] public string VarianceLinguisticPath { get; set; } = "";
    [JsonPropertyName("variance_predictor_path")] public string VariancePredictorPath { get; set; } = "";
    [JsonPropertyName("variance_tokens")] public long[]? VarianceTokens { get; set; }
    [JsonPropertyName("variance_languages")] public long[]? VarianceLanguages { get; set; }
    [JsonPropertyName("variance_speaker_embed")] public float[]? VarianceSpeakerEmbed { get; set; }
    [JsonPropertyName("variance_predicts_dur")] public bool VariancePredictsDur { get; set; }
    [JsonPropertyName("variance_predicts_energy")] public bool VariancePredictsEnergy { get; set; }
    [JsonPropertyName("variance_predicts_breathiness")] public bool VariancePredictsBreath { get; set; }
    [JsonPropertyName("variance_predicts_voicing")] public bool VariancePredictsVoicing { get; set; }
    [JsonPropertyName("variance_predicts_tension")] public bool VariancePredictsTension { get; set; }
    [JsonPropertyName("variance_continuous")] public bool VarianceContinuous { get; set; }
    [JsonPropertyName("mel_scale")] public float MelScale { get; set; } = 1;
}

sealed class VarianceResult {
    public float[]? Energy { get; init; }
    public float[]? Breathiness { get; init; }
    public float[]? Voicing { get; init; }
    public float[]? Tension { get; init; }
}
