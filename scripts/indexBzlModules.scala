//> using scala 3.7.0
//> using options --preview -Wunused:all
//> using jvm 21
//> using toolkit 0.7.0
//> using dep com.softwaremill.sttp.client4::core:4.0.2 // overrides version from toolkit
//> using dep org.apache.commons:commons-compress:1.27.1
//> using dep org.tukaani:xz:1.10 // needed for .tar.xz compressor
//> using dep org.scala-lang.modules::scala-xml:2.3.0
//> using dep com.github.scopt::scopt:4.1.0
// scalafmt: { maxColumn = 120, preset=Scala.js }

/* Script responsible for creating index of external header include to the label of extern dependency defining it's rule.
 * It does process all modules defined in https://registry.bazel.build/ and extracts information about defined header for public rules using bazel query.
 * The mapping keys are always in the normalized form of include paths that should be valid when refering using #include directive in C/C++ sources assuming include paths were not overriden
 * The values of the mapping is a string representation on Bazel label where the repository is the name of the module
 * Mappings are always based on the last version of module available in the registry. If the latest available version is yanked then whole module would be skipped.
 * 
 * The script needs to checkout (download) sources of each module and execute bazel query using a fresh instance of Bazel server.
 * This step can be ignored if .cache/modules/ contains extracted module informations from previous run.
 * 
 * When processing the results of the query script might exclude targets or headers that are assumed to be internal, the excluded files would be written in textual file on the disk.
 * Mapping contains only headers that are assigned to exactly 1 rule. Header with ambigious rule definitions are also written in textual format for manual inspection.
 * 
 * Scala runner is required for the script: either any version of Scala 3.5+ or scala-cli - it would download the required JVM and dependencies if needed.
 * See https://www.scala-lang.org/download/ or https://scala-cli.virtuslab.org/install (Scala runner is powered internally by scala-cli)
 * 
 * It does also use system binaries: git, patch (gpatch is required on MacOs instead to correctly apply patches to Bazel modules) and bazel (bazelisk preferred)
 * Usage: scala indexBzlModules.scala -- <options* | --help>
 */
@main def Index(args: String*) = {
  given config: Config = Config.resolve(args)
  val registryRepo = checkoutModulesRegistry()
  val moduleInfos = gatherModuleInfos(registryRepo)
  if config.verbose then showModuleInfos(moduleInfos)

  val mappings = createHeaderIndex(moduleInfos)
  println(s"Direct mapping created for ${mappings.headerToRule.size} headers")
  println(s"Ambigious header assignment for ${mappings.ambigious.size} entries")
  println(s"Excluded ${mappings.excluded.map(_._2.size).sum} headers in ${mappings.excluded.size} targets")
  write(config.outputPath)(mappings.headerToRule)
  write(config.cacheDir / "ambigious.json")(mappings.ambigious)
  write(config.cacheDir / "excluded.json")(mappings.excluded)
}

import upickle.default.*
import sttp.client4.*
import sttp.client4.wrappers.{FollowRedirectsBackend, TryBackend}
import sttp.model.Uri
import java.io.{FileInputStream, IOException}
import java.util.zip.{GZIPInputStream}
import org.apache.commons.compress.archivers.tar.TarArchiveInputStream
import org.apache.commons.compress.archivers.zip.ZipArchiveInputStream
import org.apache.commons.compress.compressors.xz.XZCompressorInputStream
import org.apache.commons.compress.compressors.bzip2.BZip2CompressorInputStream

import scala.util.Using
import java.io.FileOutputStream
import scala.concurrent.*
import scala.concurrent.duration.*
import java.util.concurrent.Executors
import scala.util.*
import scala.util.chaining.*

import types.*
import ModuleResolveResult.*

case class Config(
    cacheDir: os.Path = os.pwd / ".cache",
    outputPath: os.Path = os.pwd / ".cache" / "header-mappings.json",
    verbose: Boolean = false
)
object Config {
  /* Setup the config based on the list of arguments. */
  def resolve(args: Seq[String]) = {
    val default = Config()
    import scopt.*
    val builder = OParser.builder[Config]
    import builder.*
    given Read[os.Path] = summon[Read[String]].map: path =>
      Try(os.Path(path))
        .getOrElse(os.pwd / os.RelPath(path))

    val parser = OParser.sequence(
        opt[os.Path]("cache-dir")
          .action((value, c) => c.copy(cacheDir = value))
          .text(s"Path to cache directory, default: ${default.cacheDir.relativeTo(os.pwd)}"),
        opt[os.Path]("output-mappings")
          .action((value, c) => c.copy(outputPath = value))
          .text(
              s"Path were to output created header mappings index, default: ${default.outputPath.relativeTo(os.pwd)}"),
        opt[Unit]('v', "verbose")
          .action((_, c) => c.copy(verbose = true)),
        help("help").text("Show the usage of the script")
    )
    OParser
      .parse(parser, args, default)
      .getOrElse(sys.exit(1))
  }
}
// Should unresolved modules info be cached?
final val CacheUnresolvedModules = true
// Should module info be recomputed if cached result was unresolved
final val RecomputeUnresolvedModules = false
// Should resolved module sources be kept after resolving targets
final val RemoveFetchedSources = true

/**
 * Process the gathered modules infos to create a mapping of header (normalized to valid path of #include directive) to
 * the Bazel rule that defines it The header is normalized by appling values of includes / strip_include_prefix /
 * include_prefix attributes in cc_library rule
 * @param headerToRule:
 *   header include paths with exactly 1 matching rule,
 * @param ambigious
 *   header includes paths defined in multiple rules
 * @param excluded
 *   sources for given rule that are assumed to be internal, but were defined in public rule
 */
def createHeaderIndex(infos: Seq[ModuleInfo])(using config: Config): (
    headerToRule: Map[os.RelPath, Label],
    ambigious: Map[os.RelPath, Set[Label]],
    excluded: Map[Label, Set[Label]]
) = {
  // TODO: resolve conflicts
  // - if all conflicts are in the same repository try to find a commmon one
  // - if defined in different repos try to exclude internal / third_party etc directories

  import scala.collection.mutable
  val headersMapping = mutable.Map.empty[os.RelPath, mutable.Set[Label]]
  val excludedTargetEntries = mutable.Map.empty[Label, mutable.Set[Label]]
  def recordExcluded(label: Label, excluded: ModuleTarget | Label): Unit = {
    val hdrs = excluded match {
      case ModuleTarget(hdrs = hdrs) => hdrs
      case file: Label @unchecked    => List(file)
    }
    excludedTargetEntries
      .getOrElseUpdate(label, mutable.Set.empty)
      .addAll(hdrs)
  }
  // Check if file might be internal or hidden
  def shouldExcludeHeader(path: os.RelPath): Boolean = {
    standardLibraryHeaderFiles.contains(path)
    || path.toString.isBlank
    || path.segments.exists: segment =>
      Seq(".", "_").exists(segment.startsWith)
    || path.segments.headOption.exists:
      Seq("third-party", "third_party", "deps", "test").contains(_)
  }

  // Check if the the rule defining the rule is possibly internal
  def shouldExcludeTarget(target: Label): Boolean = {
    def excludeName = target.targetPath.segments.exists { segment =>
      val tokens = segment
        .split("\\W")
        .map:
          _.filter(_.isLetter)
      Seq("internal", "impl")
        .exists(tokens.contains)
    }
    def excludePkg = target.pkgRelPath.exists { path =>
      path.segments.headOption.exists(Seq("test").contains) ||
      path.segments.exists: segment =>
        Seq("third-party", "third_party", "3rd_party", "deps", "tests", "internal", "impl")
          .exists(segment.contains)
    }
    excludePkg || excludeName
  }

  // Visit all modules and collect the headers to create mapping
  // Labels are always normalized to contain the module repository
  // Excluded files are stored in the map before continuing the traversal
  for
    module <- infos
    target <- module.targets
    label = target.alias
      .filter: alias =>
        // Apply alias only if it would allow to relativize the label 
        alias.pkg.contains(alias.target) 
        || (alias.pkg.forall(_.isEmpty) && alias.target == module.module.name)
      .getOrElse(target.name)
      .withRepository(module.module.name)
    if !shouldExcludeTarget(label)
      .tapIf(_ == true)(_ => recordExcluded(label, target))
    hdr <- target.hdrs
    path <- normalizeHeaderPath(hdr.targetPath, target)
    if !shouldExcludeHeader(path)
      .tapIf(_ == true)(_ => recordExcluded(label, hdr))
    assignedLabels = headersMapping.getOrElseUpdate(path, mutable.Set.empty)
  do assignedLabels += label

  val (nonConflicting, conflicting) = headersMapping.view
    .mapValues(_.toSet)
    .toMap
    .partition { (_, labels) => labels.size == 1 }

  if config.verbose then {
    println("Modules with conflicts:")
    conflicting.values.flatten
      .flatMap(_.repository)
      .toSeq
      .distinct
      .sorted
      .zipWithIndex
      .foreach: (label, idx) =>
        println(f"${idx}%4d - ${label}")
  }

  (
      headerToRule = nonConflicting.view.mapValues(_.head).toMap,
      ambigious = conflicting,
      excluded = excludedTargetEntries.view.mapValues(_.toSet).toMap
  )
}

/**
 * Normalizes the path to the format that might be valid for C imports. It applies (strip_)include_prefix and includes
 * attributes to the format that allows the default cc_rules and C compiler to correctly resolve the header
 */
def normalizeHeaderPath(hdrPath: os.RelPath, target: ModuleTarget): Seq[os.RelPath] = {
  // Prepend target pkg to the header name, required to correctly resolve strip_include_prefix
  def targetPkgResolved(path: os.RelPath): os.RelPath = 
    target.name.pkgRelPath.foldRight(path)(_ / _)
    
  def stripIncludePrefix(path: os.RelPath): os.RelPath = 
    target.stripIncludePrefix
      .toSeq
      .flatMap: prefix =>
        Seq(prefix, targetPkgResolved(prefix))
      .find(path.startsWith)
      .foldLeft(path)(_.relativeTo(_))
      
  def includePrefix(path: os.RelPath): os.RelPath = 
      target.includePrefix.foldRight(path)(_ / _)
  
   // Relativize to the longest matching includes
  def resolveIncludes(path: os.RelPath): Seq[os.RelPath] = 
    target.includes
      .map(include => targetPkgResolved(include))
      .filter(path.startsWith)
      .map:
        case os.rel  => path
        case include => path.relativeTo(include)
      .match {
        case Nil => path :: Nil
        case paths => paths
      }
    
  targetPkgResolved
  .andThen(stripIncludePrefix)
  .andThen(resolveIncludes)
  .apply(hdrPath)
  .map(includePrefix)
}

/**
 * Visits every module listed in registry repository and extracts information about targets Runs concurrently using
 * fixed thread-pool of threads-count equal to available CPUs. We don't want to create to manny concurrent runs becouse:
 *   - we're limmited by IO when downloading module sources
 *   - we need to start a Bazel instance for each module
 *   - we need to preserve disk space and remove Bazel and no longer needed sources as soon as we get query results
 */
def gatherModuleInfos(registryRepo: os.Path)(using config: Config): List[ModuleInfo] =
  runWithFixedThreadPool {
    for
      modules = os.list(registryRepo / "modules")
      _ = println(s"Scanning ${modules.size} modules for cc_rules")
      results <- Future.traverse(modules) { module =>
        // A concurrent task for each module, parallelism limited by provided ExecutionContext in runWithFixedThreadPool
        // In verbose mode prints info about result of the query: success-rulesCount/failure-reason,
        Future:
          processModule(module)
            .tapIf(config.verbose)(logModuleResult)
      }
      // Filter out modules that we failed to resolve
      moduleInfos = results.collect:
        case info: ModuleInfo if info.targets.nonEmpty => info
      _ = println(s"Found ${moduleInfos.size} modules with non-empty cc_library defs")
      _ = println(
          s"Failed to gather module information in ${results.count(_.isInstanceOf[ModuleResolveResult.Unresolved])} modules")
    yield moduleInfos
  }
    .map(_.sortBy(_.module.toString).toList)
    .getOrElse(sys.error("Failed to gather modules info"))

// For every module it prints headers assigned to each resolved target
def showModuleInfos(moduleInfos: List[ModuleInfo]) = {
  for
    (ModuleInfo(module, targets), idx) <- moduleInfos.zipWithIndex
    _ = println(s"$idx: $module - ${targets.size}")
    target <- targets
    _ = println(s"\t${target.name}: ${target.hdrs.size} headers")
    hdr <- target.hdrs
    _ = println(s"\t\t$hdr")
  do ()
}

def logModuleResult(result: ModuleResolveResult): Unit = {
  val info = result match
    case info: ModuleInfo =>
      s"resolved - cc_libraries: ${info.targets.size}"
    case unresolved: Unresolved =>
      s"failed   - ${unresolved.reason}"
  System.err.println(f"${result.module}%-50s: $info")
  // result match
  //   case Unresolved(cause = Some(err)) => err.printStackTrace()
  //   case _ => ()
}

/**
 * Tries to collect information for Bazel registry module Selects the latest available version of module based metadata
 * Prepares (downloads) the sources of the module, applies patches and runs to bazel query to collect data Results are
 * cached by pair of (moduleName, moduleVersion)
 * @param modulePath
 *   path to bazel-registry/module/<module-name>
 */
def processModule(modulePath: os.Path)(using Config): ModuleResolveResult = {
  case class ModuleMetadata(repository: List[String], versions: List[String],
      yanked_versions: Map[String, String] = Map.empty)
      derives Reader

  val metadataFile = modulePath / "metadata.json"
  val moduleName = modulePath.last
  if !os.exists(metadataFile) then
    return Unresolved(ModuleVersion(name = moduleName, version = "invalid"), "No metadata.json")
  val metadata = read[ModuleMetadata](os.read(metadataFile))
  val latestVersion = metadata.versions.last
  val moduleVersionDir = modulePath / latestVersion
  cached(key = ModuleVersion(moduleName, latestVersion)) { module =>
    if summon[Config].verbose then println(s"Processing module $moduleName")
    val result = for
      _ <- Either.cond(
          test = !metadata.yanked_versions.contains(latestVersion),
          right = (),
          left = Unresolved(module, reason = "latest version is yanked - ignore")
      )
      (sourcesDir, projectRoot) <- prepareModuleSources(moduleVersionDir).toEither.left
        .map(err => Unresolved(module, s"Failed to prepare project sources: $err", Some(err)))
      targets <- resolveTargets(projectRoot).toEither.left
        .map(err => Unresolved(module, s"Failed to resolve module targets: $err", Some(err)))
      _ = if RemoveFetchedSources then os.remove.all(sourcesDir)
    yield ModuleInfo(
        module = module,
        targets = targets
    )
    // Left contains failure context (early exit), right resolved module info (successfull).
    // Both are a subtype of Result type so we can merge them here
    result.merge
  }
}

/**
 * Runs bazel query to find find all public cc_library rules inside project root
 * @param projectRoot
 *   path to root of directory containg MODULE.bazel
 */
def resolveTargets(projectRoot: os.Path) = Try {
  val bazelVersion: Option[(major: Int, minor: Int)] = {
    os.proc("bazel", "--batch", "version")
      .call(cwd = projectRoot, stderr = os.Pipe)
      .out
      .lines()
      .collectFirst:
        case s"Build label: $major.$minor.$_" => (major = major.toInt, minor = minor.toInt)
  }
  val tmpOutputBase = os.temp.dir()
  val selector = "//..."
  val queryResult =
    try {
      os.proc(
          "bazel",
          "--batch",
          "--max_idle_secs=5",
          s"--output_base=$tmpOutputBase",
          "query",
          s"""(kind("cc_.*library|alias", $selector) intersect attr(visibility, //visibility:public, $selector)) union kind("expand_template|filegroup", $selector)""",
          s"--output=xml",
          "--keep_going",
          "--incompatible_disallow_empty_glob=false",
          Option.when(bazelVersion.forall(_.major >= 5))("--check_direct_dependencies=off")
      ).call(cwd = projectRoot, stderr = os.Pipe, check = false)
      // Important for disk space management, each tmp dir for bazel can easily take 100 MB
    } finally
      try os.remove.all(tmpOutputBase)
      catch { case _: IOException => () }

  // Allow exitCode 3 signaling error under --keep-going
  queryResult.exitCode
    .ensuring(
        Seq(0, 3).contains(_),
        s"bazel query failed, exitCode: ${queryResult.exitCode}, version: ${bazelVersion.getOrElse("unknown")}" // stderr: ${queryResult.err.text()}"
    )

  val xmlDoc = xml.XML.loadStringDocument(queryResult.out.text())
  extractModuleTargets(xmlDoc).toList
}

/** Parses the XML output of Bazel query to extract information about cc_library */
def extractModuleTargets(doc: xml.Document): Seq[ModuleTarget] = {
  extension (nodes: xml.NodeSeq)
    def withName(name: String) = nodes.find: node =>
      (node \@ "name") == name
  extension (node: xml.Node)
    def stringOptAttr(name: String, kind: "string" | "label" | "output" = "string") =
      (node \ kind)
        .withName(name)
        .map(_ \@ "value")
    def stringListAttr(name: String, kind: "string" | "label" = "string"): List[String] =
      (node \ "list")
        .withName(name)
        .toList
        .flatMap(_ \ kind)
        .map(_ \@ "value")
  extension (value: String) def toRelPath: os.RelPath = os.RelPath(value.stripPrefix("/"))
  
  // Map of cc_library -> alias rules found in project 
  val aliases: Map[Label, Label] = {
    for 
      rule <- doc \ "rule"
      if rule \@ "class" == "alias"
      name <- Label.validate(rule \@ "name")
      target <- rule.stringOptAttr("actual", kind = "label").flatMap(Label.validate)
    yield (alias=name, target=target)
  }.groupBy(_.target)
  .collect:
    case (target, Seq(singleRef)) => target -> singleRef.alias
  .toMap
  
  val filegroups: Map[Label, Seq[Label]] = {
    for 
      rule <- doc \ "rule"
      if rule \@ "class" == "filegroup"
      name <- Label.validate(rule \@ "name")
    yield name -> rule
      .stringListAttr("srcs", kind = "label")
      .flatMap(Label.validate)
      .map(_.relativizeTo(name))
  }.toMap
  
  val expandTemplates: Map[Label, Label] = {
    for 
      rule <- doc \ "rule"
      if rule \@ "class" == "expand_template"
      name <- Label.validate(rule \@ "name")
      out <- rule
      .stringOptAttr("out", kind = "output")
      .flatMap(Label.validate)
      .map(_.relativizeTo(name))
    yield name -> out
  }.toMap
   
  for
    rule <- doc \ "rule"
    if !Seq("alias", "filegroup", "expand_template").contains(rule \@ "class")
    name <- Label.validate(rule \@ "name")
  yield ModuleTarget(
      name = name,
      alias = aliases.get(name),
      hdrs = rule
        .stringListAttr("hdrs", kind = "label")
        .flatMap(Label.validate)
        .flatMap: src => 
          filegroups.get(src)
          .orElse(expandTemplates.get(src).map(Seq(_)))
          .getOrElse(Seq(src))
        .map(_.relativizeTo(name)),
      includes = rule
        .stringListAttr("includes")
        .map(_.toRelPath),
      stripIncludePrefix = rule
        .stringOptAttr("strip_include_prefix")
        .filter(_.nonEmpty)
        .map(_.toRelPath),
      includePrefix = rule
        .stringOptAttr("include_prefix")
        .filter(_.nonEmpty)
        .map(_.toRelPath),
      deps = rule
        .stringListAttr("deps", kind = "label")
        .flatMap(Label.validate)
        .map(_.relativizeTo(name))
  )
}

case class ModuleVersion(name: String, version: String) derives ReadWriter:
  override def toString(): String = s"$name @ $version"

case class ModuleTarget(
    name: Label,
    alias: Option[Label],
    hdrs: List[Label],
    includes: List[os.RelPath],
    stripIncludePrefix: Option[os.RelPath] = None,
    includePrefix: Option[os.RelPath] = None,
    deps: List[Label]
) derives ReadWriter

enum ModuleResolveResult derives ReadWriter:
  def module: ModuleVersion
  case Unresolved(module: ModuleVersion, reason: String, cause: Option[Throwable] = None)
  case ModuleInfo(module: ModuleVersion, targets: List[ModuleTarget])

/**
 * Downloads and prepares sources of Bazel module based on metadata found in the directory It downloads the sources and
 * prepares them for further processing (unpacking, patching, etc.)
 * @param sourcesDir
 *   path to <bazel-registry>/modules/<module-name>/<module-version>/ directory containing metadata on how to retrive
 *   sources
 */
def prepareModuleSources(sourcesDir: os.Path)(
    using config: Config): Try[(sourcesDir: os.Path, projectRoot: os.Path)] = {
  val moduleSubPath = sourcesDir.relativeTo(sourcesDir / os.up / os.up)
  val targetDir = config.cacheDir / "modules" / moduleSubPath / "sources"
  if os.exists(targetDir) then os.remove.all(targetDir)

  os.makeDir.all(targetDir)
  val source = ujson.read(os.read(sourcesDir / "source.json")).obj
  source.get("type").map(_.str) match {
    case Some("git_repository") =>
      // Very rare, only 1 case git_repository usage and it's not a CC repository
      Failure(NotImplementedError("git_repository type modules are not supported yet"))
    case _ =>
      // default archive
      prepareArchiveModule(
          url = Uri.unsafeParse(source("url").str),
          stripPrefix = source.get("strip_prefix").map(_.str),
          patchStrip = source.get("patch_strip").map(_.num.toInt),
          patchFiles = source.get("patches").map(_.obj.keys.toSet).getOrElse(Set.empty),
          targetDir = targetDir,
          sourcesDir = sourcesDir
      ).map((sourcesDir = targetDir, projectRoot = _))
  }
}

/**
 * Sources processing for modules shipped using archives Downloads the archive with sources, unpacks it and applies
 * patches
 * @param url
 *   URL to the archive that should be downloaded
 * @param targetDir
 *   directory where unpacked sources should be stored
 * @param sourcesDir
 *   directory containing patches/overlays to be applied, bazel-registry/modules/<moduleName>/<moduleVersion>
 * @param stripPrefix
 *   path inside the unpacked sources where Bazel project root can be found
 * @return
 *   path to project root with prepared sources, typically `targetDir / stripPrefix`
 */
def prepareArchiveModule(url: Uri, targetDir: os.Path, sourcesDir: os.Path, stripPrefix: Option[String],
    patchStrip: Option[Int], patchFiles: Set[String]): Try[os.Path] = {
  val fileName = url.path.last
  // Download file with possible retries
  def downloadArchive(retries: Int, options: RequestOptions = basicRequest.options): Try[os.Path] =
    val artifactPath = os.temp.dir(deleteOnExit = true) / fileName
    basicRequest
      .get(url)
      .response(asPathAlways(artifactPath.toNIO))
      .withOptions(options)
      .send(TryBackend(FollowRedirectsBackend.encodeUriAll(DefaultSyncBackend())))
      .map: response =>
        os.Path(response.body)
      .recoverWith {
        case ex if retries > 0 =>
          Thread.sleep(Random.nextInt(15).seconds.toMillis)
          val newOptions = ex.getCause() match {
            case _: java.io.UnsupportedEncodingException => options.copy(decompressResponseBody = false)
            case _ => options
          }
          downloadArchive(retries - 1, newOptions)
      }
  downloadArchive(retries = 3)
    .flatMap: artifactPath =>
      extractArchive(archiveFile = artifactPath, outputDir = targetDir, stripPrefix = stripPrefix)
        .tap(_ => os.remove(artifactPath))
    .map { targetDir =>
      assert(os.exists(targetDir), s"Does not exists $targetDir")

      Option(sourcesDir / "patches")
        .filter(os.exists)
        .foreach:
          os.list(_)
            .filter: file =>
              patchFiles.contains(file.last)
            .foreach: patchFile =>
              // MacOS patch does not support renaming files - brew install gpatch
              val patchBin = if isMacOS then "gpatch" else "patch"
              val _ = os
                .proc(patchBin, s"-p${patchStrip.getOrElse(0)}", "-f", "-l", "-i", patchFile)
                .call(cwd = targetDir, stderr = os.Pipe, stdout = os.Pipe, check = true)
      Option(sourcesDir / "overlay")
        .filter(os.exists)
        .foreach:
          os.list(_)
            .foreach: file =>
              os.copy.into(file, targetDir, replaceExisting = true, createFolders = true, mergeFolders = true)
      targetDir
    }
}

def extractArchive(archiveFile: os.Path, outputDir: os.Path, stripPrefix: Option[String]): Try[os.Path] = {
  Using.Manager: use =>
    val inputStream = use(new FileInputStream(archiveFile.toIO))
    val stream: TarArchiveInputStream | ZipArchiveInputStream = archiveFile.last match {
      case s"$_.tar.gz" | s"$_.tgz" =>
        val gzipStream = use(new GZIPInputStream(inputStream))
        use(new TarArchiveInputStream(gzipStream))
      case s"$_.tar.xz" =>
        val xzStream = use(new XZCompressorInputStream(inputStream))
        use(new TarArchiveInputStream(xzStream))
      case s"$_.tar.bz2" =>
        val bz2Stream = use(new BZip2CompressorInputStream(inputStream))
        use(new TarArchiveInputStream(bz2Stream))
      case s"$_.tar" =>
        use(new TarArchiveInputStream(inputStream))
      case s"$_.zip" =>
        use(new ZipArchiveInputStream(inputStream))
    }
    if !os.exists(outputDir) then os.makeDir.all(outputDir)
    stream
      .iterator()
      .forEachRemaining { entry =>
        val outputPath = outputDir / os.SubPath(entry.getName())
        if entry.isDirectory() then os.makeDir.all(outputPath)
        else
          os.makeDir.all(outputPath / os.up)
          Using(new FileOutputStream(outputPath.toIO)) { fileOut =>
            val buffer = new Array[Byte](4 * 1024)
            Iterator
              .continually(stream.read(buffer))
              .takeWhile(_ != -1)
              .foreach: bytesRead =>
                fileOut.write(buffer, 0, bytesRead)
          }.get
      }
    stripPrefix
      .map(os.SubPath(_))
      .foldLeft(outputDir)(_ / _)
}

/**
 * Clones or updates the bazel-central-registry repository
 * @return
 *   path to the checked out repository
 */
def checkoutModulesRegistry()(using config: Config): os.Path = {
  val repoDir = config.cacheDir / "bazel-central-registry"

  if os.exists(repoDir) then
    Seq(
        "git reset --hard",
        "git checkout main",
        "git fetch origin",
        "git reset --hard origin/main"
    ).foreach: cmd =>
      os.proc(cmd.split(" ")).call(cwd = repoDir, stdout = os.Pipe, stderr = os.Inherit)
  else
    os.proc(s"git clone https://github.com/bazelbuild/bazel-central-registry --depth=1 $repoDir"
          .split(" "))
      .call(stdout = os.Pipe, stderr = os.Inherit)

  repoDir.ensuring(os.exists(repoDir), s"Modules registry not available $repoDir fater checkout")
}

object types:
  opaque type Label <: String = String
  extension (label: Label) {
    def target: String = label.split(':').last
    def targetPath: os.RelPath = os.SubPath(label.target)
    def pkg: Option[String] = Label.unapply(label).flatMap(_.pkg)
    def pkgRelPath: Option[os.RelPath] = pkg.map(_.stripPrefix("//")).map(os.RelPath(_))
    def repository = Label.unapply(label).flatMap(_.repo)
    def withRepository(repo: String): Label = s"@$repo//${label.split("//").last}"
    def relativizeTo(other: Label): Label = other.split(":") match {
      case Array(location, _) if label.startsWith(location) => label.stripPrefix(location)
      case _                                                => label
    }
  }
  object Label:
    type LabelComponents = (repo: Option[String], pkg: Option[String], target: String)
    given ReadWriter[Label] = ReadWriter.join[String].bimap(identity, identity)

    def validate(str: String): Option[Label] = unapply(str).map(_ => str)
    def unsafe(str: String): Label = str
    def unapply(str: String): Option[LabelComponents] = str match
      case s"@@$repo//$pkg:$target" => Some((repo = Some(repo), pkg = Some(pkg), target = target))
      case s"@$repo//$pkg:$target"  => Some((repo = Some(repo), pkg = Some(pkg), target = target))
      case s"//$pkg:$target"        => Some((repo = None, pkg = Some(pkg), target = target))
      case s":$target"              => Some((repo = None, pkg = None, target = target))
      case _                        => None

/**
 * Wrapes given computation into caching layer controlled by [[CacheDriver]] If the cache file exists and can be
 * deserialized returns the value, otherwise computes and stores the results in the cache directory
 * @param key
 *   unique identifier of the computation, used to calculate the cached file path
 * @param compute
 *   function to produce the value if cache is missing or corrupted. Takes the key as an input
 * @param driver
 *   typeclass defining behaviour of the cache. Controls how data is stored, decoded and how key maps to the cache file
 *   path
 * @return
 *   Deserialized cached results or computed value
 */
def cached[V, K](key: K)(compute: K => V)(using driver: CacheDriver[K, V], config: Config): V = {
  val dest = config.cacheDir / driver.destination(key)
  def computeAndCache() = {
    val result = compute(key)
    driver
      .write(result)
      .foreach: encoded =>
        os.makeDir.all(dest / os.up)
        os.write.over(dest, driver.toBytes(encoded), createFolders = true)
    result
  }

  if os.exists(dest) then
    try {
      val cachedValue = driver.load(driver.fromBytes(os.read.bytes(dest)), key)
      if !driver.invalidated(cachedValue) then cachedValue
      else
        System.err.println(s"Invalidated cached value for $key, would recompute")
        computeAndCache()
    } catch {
      case scala.util.control.NonFatal(error) =>
        System.err.println(s"Failed to load cache file: $dest, recompute, reason: ${error}")
        error.printStackTrace()
        computeAndCache()
    }
  else computeAndCache()
}

/** A driver defining how to serialized/deserialize the cached data */
trait CacheDriver[K, V]:
  type Format
  def toBytes(value: Format): Array[Byte]
  def fromBytes(data: Array[Byte]): Format

  /* Shall the cached result be recomputed */
  def invalidated(value: V): Boolean = false

  /**
   * Serialize the value
   * @return
   *   Some(serialized) data if should be cached or None if caching should be skipped
   */
  def write(value: V): Option[Format]
  /* Deserialize the entity */
  def load(data: Format, key: K): V
  /* Calculate the path to the cache file based on the key */
  def destination(key: K): os.RelPath

/** Cache driver backed by JSON format */
trait JsonCacheDriver[K, V: ReadWriter] extends CacheDriver[K, V]:
  type Format = String
  override def toBytes(value: String): Array[Byte] = value.getBytes()
  override def fromBytes(data: Array[Byte]): Format = new String(data)
  override def write(value: V): Option[String] =
    Some(upickle.default.write(value, sortKeys = true, indent = 2))
  override def load(data: String, key: K): V = upickle.default.read(data)

given JsonCacheDriver[ModuleVersion, ModuleResolveResult]:
  override def invalidated(value: ModuleResolveResult): Boolean = value match {
    case _: Unresolved => RecomputeUnresolvedModules
    case _: ModuleInfo => false // always valid
  }
  override def destination(module: ModuleVersion): os.RelPath =
    os.rel / "modules" / module.name / module.version / "module-info.json"
  override def write(value: ModuleResolveResult): Option[String] = value match {
    case _: Unresolved if !CacheUnresolvedModules => None
    case _                                        => super.write(value)
  }

/**
 * Execute given computation in the context of ExecutionContext based on fixed thread pool with thread count equal to
 * the number of available CPUs. Blocks infinetlly until computation is finished
 */
def runWithFixedThreadPool[T](
    body: ExecutionContext ?=> Future[T]
): Try[T] = {
  Using(Executors.newFixedThreadPool(Runtime.getRuntime().availableProcessors())): executor =>
    given ExecutionContext = ExecutionContext.fromExecutor(executor)
    Await.result(body
          .recover { case err: Throwable =>
            System.err.println(s"Uncought exception $err")
            err.printStackTrace()
            throw err
          }, Duration.Inf)
}

lazy val isMacOS: Boolean = sys.props("os.name").toLowerCase.contains("mac")

// Custom codecs
// the default codec for Map[String, T] seems to not be handling opaque types corectly, it creates Array[Array[String]]
given labelMapCodec[T: ReadWriter]: ReadWriter[Map[Label, T]] = ReadWriter
  .join[Map[String, T]]
  .bimap(
      _.map((key, value) => (key.toString(), value)),
      _.map((key, value) => (Label.unsafe(key), value))
  )
given ReadWriter[os.RelPath] = summon[ReadWriter[String]].bimap(
    path => path.toString,
    string => os.RelPath(string)
)
given [V: ReadWriter]: ReadWriter[Map[os.RelPath, V]] = summon[ReadWriter[Map[String, V]]].bimap(
    _.map((key, value) => (key.toString, value)),
    _.map((key, value) => (os.RelPath(key), value))
)
// No serializable
given ReadWriter[Option[Throwable]] = summon[ReadWriter[Option[Map[String, String]]]]
  .bimap(_ => None, _ => None)

def write[T: ReadWriter](path: os.Path)(value: T) = os.write
  .over(
      path,
      upickle.default.write(value, indent = 2, sortKeys = true),
      createFolders = true
  )

extension [T](value: T)
  inline def tapIf(cond: T => Boolean)(fn: T => Unit): T =
    if cond(value) then fn(value)
    value
  inline def tapIf(cond: Boolean)(fn: T => Unit): T =
    if cond then fn(value)
    value

/* scalafmt: {
     binPack.defnSite = always
     binPack.callSite = always
     newlines.configStyle.fallBack.prefer = false
   }
 */
// https://pubs.opengroup.org/onlinepubs/9799919799/idx/headers.html
lazy val posixStdlibHeaders = Set("aio.h", "arpa/inet.h", "assert.h",
  "complex.h", "cpio.h", "ctype.h", "devctl.h", "dirent.h", "dlfcn.h",
  "endian.h", "errno.h", "fcntl.h", "fenv.h", "float.h", "fmtmsg.h",
  "fnmatch.h", "ftw.h", "glob.h", "grp.h", "iconv.h", "inttypes.h", "iso646.h",
  "langinfo.h", "libgen.h", "libintl.h", "limits.h", "locale.h", "math.h",
  "monetary.h", "mqueue.h", "ndbm.h", "net/if.h", "netdb.h", "netinet/in.h",
  "netinet/tcp.h", "nl_types.h", "poll.h", "pthread.h", "pwd.h", "regex.h",
  "sched.h", "search.h", "semaphore.h", "setjmp.h", "signal.h", "spawn.h",
  "stdalign.h", "stdarg.h", "stdatomic.h", "stdbool.h", "stddef.h", "stdint.h",
  "stdio.h", "stdlib.h", "stdnoreturn.h", "string.h", "strings.h", "sys.h",
  "sys/ipc.h", "sys/cdefs.h", "sys/mman.h", "sys/msg.h", "sys/resource.h",
  "sys/select.h", "sys/sem.h", "sys/shm.h", "sys/socket.h", "sys/stat.h",
  "sys/statvfs.h", "sys/time.h", "sys/times.h", "sys/types.h", "sys/uio.h",
  "sys/un.h", "sys/utsname.h", "sys/wait.h", "syslog.h", "tar.h", "termios.h",
  "tgmath.h", "threads.h", "time.h", "uchar.h", "unistd.h", "utmpx.h",
  "wchar.h", "wctype.h", "wordexp.h")

// https://en.cppreference.com/w/c/header
val cStdLibHeaders = Set("assert.h", "complex.h", "ctype.h", "errno.h",
  "fenv.h", "float.h", "inttypes.h", "iso646.h", "limits.h", "locale.h",
  "math.h", "setjmp.h", "signal.h", "stdalign.h", "stdarg.h", "stdatomic.h",
  "stdbit.h", "stdbool.h", "stdckdint.h", "stddef.h", "stdint.h", "stdio.h",
  "stdlib.h", "stdmchar.h", "stdnoreturn.h", "string.h", "tgmath.h",
  "threads.h", "time.h", "uchar.h", "wchar.h", "wctype.h")

lazy val standardLibraryHeaderFiles =
  (cStdLibHeaders ++ posixStdlibHeaders)
    .map(os.RelPath(_))
