// NJDノードをTSVで返す。特徴量への変換はGo側で行う。

#include <cstdio>
#include <cstring>
#include <string>

// 各ヘッダは自前で extern "C" を切り替えるので、C++からはそのままincludeする。
#include "mecab.h"
#include "njd.h"
#include "text2mecab.h"
#include "mecab2njd.h"
#include "njd_set_accent_phrase.h"
#include "njd_set_accent_type.h"
#include "njd_set_digit.h"
#include "njd_set_long_vowel.h"
#include "njd_set_pronunciation.h"
#include "njd_set_unvoiced_vowel.h"

namespace {

Mecab g_mecab;
NJD g_njd;
bool g_initialized = false;
bool g_loaded = false;
char g_error[512];

void set_error(const char* message) {
  std::snprintf(g_error, sizeof(g_error), "%s", message);
}

const char* safe(const char* value) { return value != nullptr ? value : ""; }

void append_escaped(std::string& out, const char* value) {
  for (const char* p = safe(value); *p != '\0'; ++p) {
    switch (*p) {
      case '\\': out += "\\\\"; break;
      case '\t': out += "\\t"; break;
      case '\n': out += "\\n"; break;
      case '\r': out += "\\r"; break;
      default: out += *p; break;
    }
  }
}

void append_escaped_field(std::string& out, const char* value) {
  append_escaped(out, value);
  out += '\t';
}

}  // namespace

extern "C" {

// 成功は1、失敗は0。
int UtauTTSOpenJTalkInit(const char* dictionary_path) {
  if (dictionary_path == nullptr || dictionary_path[0] == '\0') {
    set_error("dictionary path is empty");
    return 0;
  }
  if (!g_initialized) {
    Mecab_initialize(&g_mecab);
    NJD_initialize(&g_njd);
    g_initialized = true;
  }
  if (!Mecab_load(&g_mecab, dictionary_path)) {
    g_loaded = false;
    set_error("failed to load Open JTalk dictionary");
    return 0;
  }
  g_loaded = true;
  return 1;
}

// 戻り値はTSVのバイト数（NULを除く）。失敗は0。
// 列: string, pos, pos_group1, ctype, cform, orig, read, pron, acc, mora_size, chain_rule, chain_flag
int UtauTTSOpenJTalkRun(const char* text, char* output, int output_capacity) {
  if (!g_loaded) {
    set_error("dictionary is not loaded");
    return 0;
  }
  if (text == nullptr || output == nullptr || output_capacity <= 0) {
    set_error("invalid Open JTalk buffers");
    return 0;
  }
  char buffer[8192];
  text2mecab(buffer, text);
  if (!Mecab_analysis(&g_mecab, buffer)) {
    set_error("MeCab analysis failed");
    return 0;
  }
  mecab2njd(&g_njd, Mecab_get_feature(&g_mecab), Mecab_get_size(&g_mecab));
  njd_set_pronunciation(&g_njd);
  njd_set_digit(&g_njd);
  njd_set_accent_phrase(&g_njd);
  njd_set_accent_type(&g_njd);
  njd_set_unvoiced_vowel(&g_njd);
  njd_set_long_vowel(&g_njd);

  std::string out;
  for (NJDNode* node = g_njd.head; node != nullptr; node = node->next) {
    append_escaped_field(out, NJDNode_get_string(node));
    append_escaped_field(out, NJDNode_get_pos(node));
    append_escaped_field(out, NJDNode_get_pos_group1(node));
    append_escaped_field(out, NJDNode_get_ctype(node));
    append_escaped_field(out, NJDNode_get_cform(node));
    append_escaped_field(out, NJDNode_get_orig(node));
    append_escaped_field(out, NJDNode_get_read(node));
    append_escaped_field(out, NJDNode_get_pron(node));
    out += std::to_string(NJDNode_get_acc(node));
    out += '\t';
    out += std::to_string(NJDNode_get_mora_size(node));
    out += '\t';
    append_escaped_field(out, NJDNode_get_chain_rule(node));
    out += std::to_string(NJDNode_get_chain_flag(node));
    out += '\n';
  }

  NJD_refresh(&g_njd);
  Mecab_refresh(&g_mecab);

  if (static_cast<int>(out.size()) + 1 > output_capacity) {
    set_error("output buffer is too small");
    return 0;
  }
  std::memcpy(output, out.data(), out.size());
  output[out.size()] = '\0';
  return static_cast<int>(out.size());
}

const char* UtauTTSOpenJTalkError(void) { return g_error; }

}  // extern "C"
