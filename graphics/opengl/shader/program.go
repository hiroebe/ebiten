package shader

// #cgo LDFLAGS: -framework OpenGL
//
// #include <OpenGL/gl.h>
// #include <stdlib.h>
import "C"
import (
	"github.com/hajimehoshi/go-ebiten/graphics/matrix"
	"github.com/hajimehoshi/go-ebiten/graphics/texture"
	"unsafe"
)

func createProgram(shaders ...*shader) C.GLuint {
	program := C.glCreateProgram()
	for _, shader := range shaders {
		C.glAttachShader(program, shader.id)
	}
	C.glLinkProgram(program)
	linked := C.GLint(C.GL_FALSE)
	C.glGetProgramiv(program, C.GL_LINK_STATUS, &linked)
	if linked == C.GL_FALSE {
		panic("program error")
	}
	return program
}

var (
	initialized = false
)

func initialize() {
	// TODO: when should this function be called?
	vertexShader.id = C.glCreateShader(C.GL_VERTEX_SHADER)
	if vertexShader.id == 0 {
		panic("creating shader failed")
	}
	fragmentShader.id = C.glCreateShader(C.GL_FRAGMENT_SHADER)
	if fragmentShader.id == 0 {
		panic("creating shader failed")
	}
	colorMatrixShader.id = C.glCreateShader(C.GL_FRAGMENT_SHADER)
	if colorMatrixShader.id == 0 {
		panic("creating shader failed")
	}

	vertexShader.compile()
	fragmentShader.compile()
	colorMatrixShader.compile()

	programRegular = createProgram(vertexShader, fragmentShader)
	programColorMatrix = createProgram(vertexShader, colorMatrixShader)

	C.glDeleteShader(vertexShader.id)
	C.glDeleteShader(fragmentShader.id)
	C.glDeleteShader(colorMatrixShader.id)

	initialized = true
}

const (
	qualifierVariableTypeAttribute = iota
	qualifierVariableTypeUniform
)

var (
	shaderLocationCache = map[int]map[string]C.GLint{
		qualifierVariableTypeAttribute: map[string]C.GLint{},
		qualifierVariableTypeUniform:   map[string]C.GLint{},
	}
)

func getLocation(program C.GLuint, name string, qualifierVariableType int) C.GLint {
	if location, ok := shaderLocationCache[qualifierVariableType][name]; ok {
		return location
	}

	locationName := C.CString(name)
	defer C.free(unsafe.Pointer(locationName))

	location := C.GLint(-1)
	
	switch qualifierVariableType {
	case qualifierVariableTypeAttribute:
		location = C.glGetAttribLocation(program, (*C.GLchar)(locationName))
	case qualifierVariableTypeUniform:
		location = C.glGetUniformLocation(program, (*C.GLchar)(locationName))
	default:
		panic("no reach")
	}
	if location == -1 {
		panic("glGetUniformLocation failed")
	}
	shaderLocationCache[qualifierVariableType][name] = location

	return location
}

func getAttributeLocation(program C.GLuint, name string) C.GLint {
	return getLocation(program, name, qualifierVariableTypeAttribute)
}

func getUniformLocation(program C.GLuint, name string) C.GLint {
	return getLocation(program, name, qualifierVariableTypeUniform)
}

func use(projectionMatrix [16]float32, geometryMatrix matrix.Geometry, colorMatrix matrix.Color) C.GLuint {
	program := programRegular
	if !colorMatrix.IsIdentity() {
		program = programColorMatrix
	}
	C.glUseProgram(program)
	
	C.glUniformMatrix4fv(C.GLint(getUniformLocation(program, "projection_matrix")),
		1, C.GL_FALSE, (*C.GLfloat)(&projectionMatrix[0]))

	a := float32(geometryMatrix.Elements[0][0])
	b := float32(geometryMatrix.Elements[0][1])
	c := float32(geometryMatrix.Elements[1][0])
	d := float32(geometryMatrix.Elements[1][1])
	tx := float32(geometryMatrix.Elements[0][2])
	ty := float32(geometryMatrix.Elements[1][2])
	glModelviewMatrix := [...]float32{
		a, c, 0, 0,
		b, d, 0, 0,
		0, 0, 1, 0,
		tx, ty, 0, 1,
	}
	C.glUniformMatrix4fv(getUniformLocation(program, "modelview_matrix"),
		1, C.GL_FALSE,
		(*C.GLfloat)(&glModelviewMatrix[0]))

	C.glUniform1i(getUniformLocation(program, "texture"), 0)

	if program != programColorMatrix {
		return program
	}

	e := [4][5]float32{}
	for i := 0; i < 4; i++ {
		for j := 0; j < 5; j++ {
			e[i][j] = float32(colorMatrix.Elements[i][j])
		}
	}

	glColorMatrix := [...]float32{
		e[0][0], e[1][0], e[2][0], e[3][0],
		e[0][1], e[1][1], e[2][1], e[3][1],
		e[0][2], e[1][2], e[2][2], e[3][2],
		e[0][3], e[1][3], e[2][3], e[3][3],
	}
	C.glUniformMatrix4fv(getUniformLocation(program, "color_matrix"),
		1, C.GL_FALSE, (*C.GLfloat)(&glColorMatrix[0]))

	glColorMatrixTranslation := [...]float32{
		e[0][4], e[1][4], e[2][4], e[3][4],
	}
	C.glUniform4fv(getUniformLocation(program, "color_matrix_translation"),
		1, (*C.GLfloat)(&glColorMatrixTranslation[0]))

	return program
}

type Texture C.GLuint

func DrawTexture(native Texture, projectionMatrix [16]float32, quads []texture.Quad,
	geometryMatrix matrix.Geometry, colorMatrix matrix.Color) {
	if !initialized {
		initialize()
	}

	if len(quads) == 0 {
		return
	}
	shaderProgram := use(projectionMatrix, geometryMatrix, colorMatrix)
	// This state affects the other functions, so can't disable here...
	C.glBindTexture(C.GL_TEXTURE_2D, C.GLuint(native))
	defer C.glBindTexture(C.GL_TEXTURE_2D, 0)

	vertexAttrLocation := getAttributeLocation(shaderProgram, "vertex")
	textureAttrLocation := getAttributeLocation(shaderProgram, "texture")

	C.glEnableClientState(C.GL_VERTEX_ARRAY)
	C.glEnableClientState(C.GL_TEXTURE_COORD_ARRAY)
	C.glEnableVertexAttribArray(C.GLuint(vertexAttrLocation))
	C.glEnableVertexAttribArray(C.GLuint(textureAttrLocation))
	defer func() {
		C.glDisableVertexAttribArray(C.GLuint(textureAttrLocation))
		C.glDisableVertexAttribArray(C.GLuint(vertexAttrLocation))
		C.glDisableClientState(C.GL_TEXTURE_COORD_ARRAY)
		C.glDisableClientState(C.GL_VERTEX_ARRAY)
	}()

	vertices := []float32{}
	texCoords := []float32{}
	indicies := []uint32{}
	// TODO: Check len(parts) and GL_MAX_ELEMENTS_INDICES
	for i, quad := range quads {
		x1 := quad.VertexX1
		x2 := quad.VertexX2
		y1 := quad.VertexY1
		y2 := quad.VertexY2
		vertices = append(vertices,
			x1, y1,
			x2, y1,
			x1, y2,
			x2, y2,
		)
		u1 := quad.TextureCoordU1
		u2 := quad.TextureCoordU2
		v1 := quad.TextureCoordV1
		v2 := quad.TextureCoordV2
		texCoords = append(texCoords,
			u1, v1,
			u2, v1,
			u1, v2,
			u2, v2,
		)
		base := uint32(i * 4)
		indicies = append(indicies,
			base, base+1, base+2,
			base+1, base+2, base+3,
		)
	}
	C.glVertexAttribPointer(C.GLuint(vertexAttrLocation), 2,
		C.GL_FLOAT, C.GL_FALSE,
		0, unsafe.Pointer(&vertices[0]))
	C.glVertexAttribPointer(C.GLuint(textureAttrLocation), 2,
		C.GL_FLOAT, C.GL_FALSE,
		0, unsafe.Pointer(&texCoords[0]))
	C.glDrawElements(C.GL_TRIANGLES, C.GLsizei(len(indicies)),
		C.GL_UNSIGNED_INT, unsafe.Pointer(&indicies[0]))
}
