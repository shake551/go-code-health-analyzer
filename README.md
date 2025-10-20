# Go Code Health Analyzer

Go言語プロジェクトのコード品質を多角的に分析し、インタラクティブなHTMLレポートを生成するコマンドラインツールです。

## 概要

このツールは以下の3つの重要な指標を統合的に計測します：

1. **LCOM4 (Lack of Cohesion of Methods)** - 構造体の凝集度
   - 構造体内の責務のまとまりを評価
   - スコアが低いほど良好（1が理想的）

2. **循環的複雑度 (Cyclomatic Complexity)** - 関数の複雑度
   - 関数/メソッドの論理的な複雑さを評価
   - スコアが低いほど保守しやすい

3. **結合度 (Coupling)** - パッケージ間の依存関係
   - パッケージ間の依存関係の強さを評価
   - 不安定度(Instability)で測定

## インストール

```bash
# プロジェクトをクローン
git clone https://github.com/hiroki-yamauchi/go-code-health-analyzer.git
cd go-code-health-analyzer

# ビルド
go build -o go-code-health-analyzer

# （オプション）インストール
go install
```

## 使い方

### 基本的な使い方

```bash
# カレントディレクトリを解析（HTML形式）
./go-code-health-analyzer .

# 特定のディレクトリを解析
./go-code-health-analyzer /path/to/your/project

# JSON形式で出力
./go-code-health-analyzer -format json ./myproject

# HTMLとJSON両方を出力
./go-code-health-analyzer -format both ./myproject

# カスタムファイル名を指定
./go-code-health-analyzer -format json -output report.json ./myproject

# 特定のディレクトリを除外
./go-code-health-analyzer -exclude "build,dist,tmp" ./myproject

# ネストされたパスを除外
./go-code-health-analyzer -exclude "internal/generated,pkg/old/legacy" ./myproject

# 複数のオプションを組み合わせる
./go-code-health-analyzer -format json -exclude "node_modules,build" -output report.json ./myproject
```

### オプション

- `-format`: 出力形式を指定（`html`, `json`, `both`）デフォルト: `html`
- `-output`: 出力ファイルのパスを指定。デフォルト: `code_health_report.html` または `code_health_report.json`
- `-exclude`: 解析から除外するディレクトリをカンマ区切りで指定
  - ディレクトリ名（例：`build`, `dist`）またはパス（例：`internal/generated`, `pkg/old/legacy`）を指定可能
  - デフォルトで `vendor` と `testdata` は常に除外されます
  - 隠しディレクトリ（`.`で始まる）も常に除外されます

### 出力形式

#### HTML形式（デフォルト）

解析が完了すると、`code_health_report.html` が生成されます。このファイルをブラウザで開くことで、インタラクティブなレポートを閲覧できます。

#### JSON形式

`-format json` を指定すると、`code_health_report.json` が生成されます。このJSON形式の出力は、以下のような用途に利用できます：

- CI/CDパイプラインでの品質チェック
- カスタムツールやスクリプトでの分析
- 時系列でのメトリクス推移の追跡
- 他のツールとの連携

## レポート機能

生成されるHTMLレポートには以下の機能があります：

### サマリーセクション
- プロジェクト全体の統計情報
- 要注意項目の数（高LCOM4、高複雑度、高不安定度）

### パッケージ結合度タブ
- Ca (Afferent Coupling): このパッケージに依存しているパッケージ数
- Ce (Efferent Coupling): このパッケージが依存しているパッケージ数
- Instability (不安定度): Ce / (Ca + Ce)
- クリックで他のタブをフィルタリング

### 構造体凝集度タブ
- 各構造体のLCOM4スコア
- パッケージでフィルタリング可能
- 色分け: 緑(1)、黄(2)、赤(3+)

### 関数複雑度タブ
- 各関数の循環的複雑度
- パッケージでフィルタリング可能
- 色分け: 緑(1-10)、黄(11-15)、赤(16+)

### インタラクティブ機能
- テーブルのソート（各列をクリック）
- パッケージによるフィルタリング
- 色分けによる視覚的な問題箇所の識別

## 評価基準

### LCOM4
- **1 (緑)**: 理想的な凝集度
- **2 (黄)**: 注意が必要
- **3+ (赤)**: リファクタリングを推奨

### 循環的複雑度
- **1-10 (緑)**: シンプルで保守しやすい
- **11-15 (黄)**: やや複雑
- **16+ (赤)**: 複雑すぎる、リファクタリング推奨

### 不安定度
- **0-0.3 (緑)**: 安定している
- **0.3-0.7 (黄)**: 中程度
- **0.7-1.0 (赤)**: 不安定、変更の影響が大きい

## プロジェクト構造

```
go-code-health-analyzer/
├── main.go                 # CLIエントリーポイント
├── go.mod                  # Go モジュール定義
├── analyzer/               # 解析エンジン
│   ├── types.go           # データ構造定義
│   ├── lcom4.go           # LCOM4計算
│   ├── complexity.go      # 循環的複雑度計算
│   ├── coupling.go        # 結合度計算
│   └── analyzer.go        # メイン解析ロジック
└── reporter/              # レポート生成
    ├── reporter.go        # レポート生成ロジック
    └── template.html      # HTMLテンプレート
```

## 技術仕様

- **言語**: Go 1.21+
- **標準ライブラリ使用**:
  - `go/parser`, `go/ast`, `go/token` - ソースコード解析
  - `html/template` - HTMLレポート生成
- **外部依存**: なし（標準ライブラリのみ）
- **フロントエンド**:
  - Tailwind CSS (CDN経由)
  - Vanilla JavaScript
