export type Session = {
  slug: string;
  num: string;
  titleZh: string;
  titleEn: string;
  upstream: string;
  available: boolean;
};

export const SESSIONS: Session[] = [
  { slug: "s01-loop",     num: "s01",     titleZh: "最小 Agent 主循环",       titleEn: "The Minimal Agent Loop",          upstream: "agent_loop.py:agent_runner_loop", available: true  },
  { slug: "s02-tools",    num: "s02",     titleZh: "工具注册表与 dispatch",    titleEn: "Tool Registry & Dispatch",        upstream: "agent_loop.py:BaseHandler",      available: true  },
  { slug: "s03-outcome",  num: "s03",     titleZh: "StepOutcome 控制流",       titleEn: "StepOutcome Control Flow",        upstream: "agent_loop.py:StepOutcome",      available: true  },
  { slug: "s04-claude",   num: "s04",     titleZh: "真实 Anthropic Claude",   titleEn: "Real Anthropic Claude Provider",  upstream: "llmcore.py:NativeClaudeSession", available: true  },
  { slug: "s05-coderun",  num: "s05",     titleZh: "流式代码执行工具",         titleEn: "Streaming Code Execution",         upstream: "ga.py:code_run",                 available: true  },
  { slug: "s06-fileops",  num: "s06",     titleZh: "文件读写补丁工具",         titleEn: "File Read / Write / Patch",       upstream: "ga.py:file_read/write/patch",    available: false },
  { slug: "s07-memory",   num: "s07",     titleZh: "分层记忆与 working ckpt",  titleEn: "Layered Memory + Checkpoint",     upstream: "memory/ + ga.py",                available: false },
  { slug: "s08-skills",   num: "s08",     titleZh: "技能树与技能搜索",         titleEn: "Skill Tree & Skill Search",       upstream: "memory/skill_search/SKILL.md",   available: false },
  { slug: "s09-mixin",    num: "s09",     titleZh: "多 provider 故障切换",     titleEn: "Multi-Provider Failover",         upstream: "llmcore.py:MixinSession",        available: false },
  { slug: "s10-reflect",  num: "s10",     titleZh: "反射模式与自动调度",       titleEn: "Reflect Mode & Scheduling",       upstream: "agentmain.py --reflect",         available: false },
  { slug: "s_full",       num: "s_full",  titleZh: "集成端到端用例",           titleEn: "End-to-end Integration",          upstream: "agentmain.py",                   available: false },
  { slug: "appendix-a-self-evolution", num: "附录 A", titleZh: "自演化的本质", titleEn: "The essence of self-evolution",   upstream: "memory/ + README",               available: false },
  { slug: "appendix-b-upstream-map",   num: "附录 B", titleZh: "上游源码导读", titleEn: "Upstream source-reading map",     upstream: "whole repo",                     available: false },
];
