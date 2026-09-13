from __future__ import annotations

"""题型语义和答案约定，供 OCR、解析和追问共用。"""

QUESTION_TYPE_GUIDANCE: dict[str, str] = {
    "single_choice": "单选题：answer 必须是一个存在的选项 key；optionAnalysis 逐项解释，不要拼接多个选项。",
    "multiple_choice": "多选题：answer 使用逗号分隔的选项 key（例如 A,C），按选项顺序输出，并逐项说明是否成立。",
    "true_false": "判断题：answer 只能是 true 或 false；summary 说明判断依据，不要按普通单选题处理。",
    "fill_blank": "填空题：answer 按空位顺序使用分号分隔；blankAnswers 返回每个空的答案，不生成选项分析。",
    "subjective": "主观题：answer 必须直接给出可独立阅读的完整参考作答，并逐项满足题干要求，不能只给结论或写‘见解析’；题目要求算法、证明、代码或伪代码时，answer 必须包含对应内容。scoringPoints 和 rubric 仅作评分参考，不能代替答案，不声称最终判分。",
    "short_answer": "简答题：answer 给出完整精炼的参考答案，scoringPoints 列出关键要点，不套用选项分析。",
    "essay": "论述题：answer 给出一份完整、可独立阅读的参考作答，而不是只列作答方向；steps 表示论证结构，scoringPoints 和 rubric 供人工核对。",
    "calculation": "计算题：answer 给出最终结果和单位，steps 展示公式、代入和中间结论，scoringPoints 列出关键计算步骤。",
    "unknown": "题型暂不确定：只根据题面证据给出保守结果，并标记需要人工校准。",
}


def question_type_guidance(question_type: str) -> str:
    return QUESTION_TYPE_GUIDANCE.get(question_type, QUESTION_TYPE_GUIDANCE["unknown"])


def infer_question_type(stem: str, raw_text: str, has_options: bool) -> str:
    """只做低风险规则推断，无法确认时交给用户或 LLM 校准。"""
    text = f"{stem}\n{raw_text}".lower()
    if has_options:
        if any(marker in text for marker in ("多选", "多项选择", "不定项选择")):
            return "multiple_choice"
        if any(marker in text for marker in ("判断题", "正确或错误", "对或错", "正确/错误")):
            return "true_false"
        return "single_choice"
    if any(marker in text for marker in ("填空题", "请填入", "括号中", "____", "（ ）", "( )", "（）")):
        return "fill_blank"
    if any(marker in text for marker in ("计算题", "求出", "求解", "计算", "证明")):
        return "calculation"
    if any(marker in text for marker in ("论述题", "作文", "论述")):
        return "essay"
    if any(marker in text for marker in ("简答题", "简述", "简答")):
        return "short_answer"
    return "subjective"
